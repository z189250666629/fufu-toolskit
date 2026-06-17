import { useCallback, useEffect, useRef, useState } from 'react';
import { Button, Input } from '@heroui/react';
import { messageFromError } from '../api';
import { MessageLine } from '../blueprint';
import { Metric, type MessageState } from './adminShared';
import { SALE_SLOTS, buildSaleCardSchedule, buildSlotState, validateTargetStock, type SlotState } from './saleCardConfigCore';
import type { SaleCardConfig, SaleCardRunResult, SaleCardStockResponse } from './types';

const STOCK_REFRESH_INTERVAL_MS = 5 * 60 * 1000;

export function SaleCardManager({
  config,
  onSave,
  onRun
}: {
  config?: SaleCardConfig;
  onSave: (schedule: NonNullable<SaleCardConfig['schedule']>) => Promise<void>;
  onRun: (plan: string, targetStock: number) => Promise<SaleCardRunResult>;
}) {
  const plans = config?.plans ?? [];
  const [enabled, setEnabled] = useState(Boolean(config?.schedule?.enabled));
  const [timezone, setTimezone] = useState(config?.schedule?.timezone || 'Asia/Shanghai');
  const [slotState, setSlotState] = useState<Record<string, SlotState>>(() => buildSlotState(config));
  const [runPlan, setRunPlan] = useState(plans[0]?.id ?? '');
  const [runTarget, setRunTarget] = useState(50);
  const [runResult, setRunResult] = useState<SaleCardRunResult>();
  const [runMessage, setRunMessage] = useState<MessageState>({ text: '' });
  const [scheduleMessage, setScheduleMessage] = useState<MessageState>({ text: '' });
  const [running, setRunning] = useState(false);
  const [stock, setStock] = useState<Record<string, number>>({});
  const [refreshing, setRefreshing] = useState(false);
  const [stockError, setStockError] = useState('');
  const refreshingRef = useRef(false);

  useEffect(() => {
    setEnabled(Boolean(config?.schedule?.enabled));
    setTimezone(config?.schedule?.timezone || 'Asia/Shanghai');
    setSlotState(buildSlotState(config));
    setRunPlan((config?.plans ?? [])[0]?.id ?? '');
  }, [config]);

  const refreshStock = useCallback(async () => {
    if (refreshingRef.current) return;
    refreshingRef.current = true;
    setRefreshing(true);
    setStockError('');
    try {
      const response = await fetch('/api/admin/sale-cards/stock', { credentials: 'same-origin', cache: 'no-store' });
      const body = (await response.json().catch(() => ({}))) as Partial<SaleCardStockResponse> & { error?: string };
      if (!response.ok) {
        throw new Error(body.error || `查询库存失败（${response.status}）`);
      }
      const map: Record<string, number> = {};
      for (const entry of body.stock ?? []) map[entry.planId] = entry.currentStock;
      setStock(map);
    } catch (error) {
      setStockError(error instanceof Error ? error.message : '查询库存失败');
    } finally {
      refreshingRef.current = false;
      setRefreshing(false);
    }
  }, []);

  const planIds = plans.map((plan) => plan.id).join('|');

  useEffect(() => {
    if (!planIds) return;
    void refreshStock();
    const timer = window.setInterval(() => {
      void refreshStock();
    }, STOCK_REFRESH_INTERVAL_MS);
    return () => window.clearInterval(timer);
  }, [planIds, refreshStock]);

  const stockText = (planId: string) => (planId in stock ? String(stock[planId]) : '—');

  function patchSlot(group: string, patch: Partial<Omit<SlotState, 'jobs'>>) {
    setSlotState((current) => ({ ...current, [group]: { ...current[group], ...patch } }));
  }
  function patchJob(group: string, planId: string, patch: Partial<{ targetStock: number; enabled: boolean }>) {
    setSlotState((current) => {
      const slot = current[group];
      const job = slot.jobs[planId] ?? { targetStock: 0, enabled: false };
      return { ...current, [group]: { ...slot, jobs: { ...slot.jobs, [planId]: { ...job, ...patch } } } };
    });
  }
  async function saveSchedule() {
    setScheduleMessage({ text: '正在保存补卡计划…' });
    try {
      await onSave(buildSaleCardSchedule(enabled, timezone, slotState, plans));
      setScheduleMessage({ text: '补卡计划已保存', tone: 'ok' });
    } catch (error) {
      setScheduleMessage({ text: messageFromError(error, '补卡计划保存失败'), tone: 'error' });
    }
  }

  async function runNow() {
    if (!runPlan) {
      setRunMessage({ text: '请选择要补的卡种。', tone: 'error' });
      return;
    }
    const target = validateTargetStock(runTarget);
    if (!target.ok) {
      setRunMessage({ text: target.message ?? '补卡目标库存无效。', tone: 'error' });
      return;
    }
    setRunning(true);
    setRunResult(undefined);
    setRunMessage({ text: '正在执行补卡，请等待生成卡密并上架到 MCY…' });
    try {
      const result = await onRun(runPlan, target.target);
      if (result) {
        setRunResult(result);
        const nextStock = (result.currentStock ?? 0) + (result.uploaded ?? 0);
        setStock((current) => ({ ...current, [runPlan]: result.targetStock || nextStock || current[runPlan] || 0 }));
        setRunMessage({ text: `补卡完成：本次补 ${result.toUpload ?? 0} 张，已上架 ${result.uploaded ?? 0} 张。`, tone: 'ok' });
      } else {
        setRunMessage({ text: '补卡没有完成。', tone: 'error' });
      }
    } catch (error) {
      setRunMessage({ text: messageFromError(error, '补卡执行失败'), tone: 'error' });
    } finally {
      setRunning(false);
    }
  }

  return (
    <div className="business-stack">
      <div className="action-row">
        <label className="field--inline"><input type="checkbox" checked={enabled} onChange={(event) => setEnabled(event.target.checked)} /> 启用自动补卡</label>
        <label className="field--inline field--inline-tz">时区<Input className="mini-input blueprint-input" value={timezone} onChange={(event) => setTimezone(event.target.value)} aria-label="时区" /></label>
        <Button className="blueprint-button" onPress={() => { void refreshStock(); }} isDisabled={refreshing}>{refreshing ? '刷新中…' : '刷新当前库存'}</Button>
        <Button className="blueprint-primary-button" onPress={saveSchedule}>保存补卡计划</Button>
      </div>
      <p className="inline-help">月次卡与 55 次混合特惠卡各占一个独立时段；到点按目标库存补齐（补 目标-当前）。点“刷新当前库存”查询 MCY 商城实时可用卡量。</p>
      {scheduleMessage.text ? <MessageLine tone={scheduleMessage.tone}>{scheduleMessage.text}</MessageLine> : null}
      {stockError ? <MessageLine tone="error">{stockError}</MessageLine> : null}
      {SALE_SLOTS.map((def) => {
        const slot = slotState[def.group];
        const slotPlans = plans.filter((plan) => (plan.slot || '') === def.group);
        if (!slot) return null;
        return (
          <section key={def.group} className="bp-card sale-slot-card">
            <header className="bp-card-titlebar">
              <span>{def.label} 时段</span>
              <div className="sale-slot-controls">
                <Input className="mini-input blueprint-input sale-time-input" type="time" value={slot.time} onChange={(event) => patchSlot(def.group, { time: event.target.value })} aria-label={`${def.label}补卡时间`} />
                <label className="field--inline"><input type="checkbox" checked={slot.enabled} onChange={(event) => patchSlot(def.group, { enabled: event.target.checked })} /> 启用时段</label>
              </div>
            </header>
            <div className="bp-card-body">
              <div className="sale-job-list">
                {slotPlans.length ? slotPlans.map((plan) => {
                  const job = slot.jobs[plan.id] ?? { targetStock: 0, enabled: false };
                  return (
                    <div className="sale-job-row" key={plan.id}>
                      <span className="sale-job-name">{plan.name || plan.id}<span className="sale-job-meta">SKU {plan.skuId || '-'} · ${plan.quota ?? '-'}</span></span>
                      <span className="sale-job-current">当前 <b>{stockText(plan.id)}</b></span>
                      <label className="field--inline sale-job-target">补齐到<Input className="mini-input blueprint-input" type="number" min={0} max={2000} value={String(job.targetStock || '')} onChange={(event) => patchJob(def.group, plan.id, { targetStock: Number(event.target.value) })} aria-label={`${plan.name || plan.id}目标库存`} />张</label>
                      <label className="field--inline"><input type="checkbox" checked={job.enabled} onChange={(event) => patchJob(def.group, plan.id, { enabled: event.target.checked })} /> 启用</label>
                    </div>
                  );
                }) : <p className="inline-help">该时段暂无计划。</p>}
              </div>
            </div>
          </section>
        );
      })}
      <div className="config-subhead">立即补卡（手动）</div>
      <div className="action-row">
        <select className="native-select" value={runPlan} onChange={(event) => setRunPlan(event.target.value)}>
          {plans.length ? plans.map((plan) => <option key={plan.id} value={plan.id}>{plan.name || plan.id}</option>) : <option value="">暂无计划</option>}
        </select>
        <span className="sale-job-current">当前 <b>{stockText(runPlan)}</b></span>
        <label className="field--inline">补齐到<Input className="mini-input blueprint-input" type="number" min={0} max={2000} value={String(runTarget)} onChange={(event) => setRunTarget(Number(event.target.value))} aria-label="目标库存" />张</label>
        <Button className="blueprint-primary-button" onPress={runNow} isDisabled={!runPlan || running}>{running ? '补卡中…' : '立即补卡'}</Button>
      </div>
      {runMessage.text ? <MessageLine tone={runMessage.tone}>{runMessage.text}</MessageLine> : null}
      {runResult ? (
        <div className="metrics sale-run-result">
          <Metric label="当前库存" value={runResult.currentStock} />
          <Metric label="目标库存" value={runResult.targetStock} />
          <Metric label="本次补卡" value={runResult.toUpload} />
          <Metric label="已上架" value={runResult.uploaded} />
        </div>
      ) : null}
    </div>
  );
}
