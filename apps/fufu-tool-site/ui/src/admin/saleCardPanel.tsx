import { MessageLine } from '../blueprint';
import { Metric } from './adminShared';
import { SALE_SLOTS } from './saleCardConfigCore';
import type { SaleCardConfig, SaleCardRestockJobStatus } from './types';

const STATUS_LABELS: Record<string, string> = {
  pending: '等待补卡',
  running: '补卡中',
  succeeded: '今日已补齐',
  failed: '补卡失败'
};

export function SaleCardManager({ config }: { config?: SaleCardConfig }) {
  const plans = config?.plans ?? [];
  const savedSchedule = config?.schedule;
  const savedEnabled = Boolean(savedSchedule?.enabled || savedSchedule?.slots?.some((slot) => slot.enabled));
  const jobs = config?.restockStatus?.jobs ?? [];

  const slotName = (group?: string) => SALE_SLOTS.find((slot) => slot.group === group)?.label ?? group ?? '-';
  const planName = (planId: string) => plans.find((plan) => plan.id === planId)?.name || planId;

  return (
    <div className="business-stack">
      <MessageLine tone={jobs.some((job) => job.status === 'failed') ? 'error' : undefined}>自动补卡按已保存计划执行；任务超时会取消请求、复查 MCY 库存并按 DB 任务单独重试。</MessageLine>
      <p className="inline-help">后台不做全天库存监控；只在配置时段入队当天任务，失败原因会记录在这里。</p>
      <div className="metrics sale-run-result">
        <Metric label="卡档数量" value={plans.length} />
        <Metric label="自动计划" value={savedEnabled ? '已启用' : '未启用'} />
        <Metric label="失败任务" value={jobs.filter((job) => job.status === 'failed').length} />
      </div>
      <div className="config-subhead">补卡任务状态</div>
      <div className="sale-job-list">
        {jobs.length ? jobs.map((job) => <SaleCardRestockRow key={job.id} job={job} planName={planName(job.planId)} slotName={slotName(job.slotGroup)} />) : (
          <p className="inline-help">暂无补卡任务记录；到配置时段后会自动生成当天任务。</p>
        )}
      </div>
      <div className="config-subhead">卡档参考</div>
      <div className="sale-job-list">
        {plans.length ? plans.map((plan) => (
          <div className="sale-job-row" key={plan.id}>
            <span className="sale-job-name">
              {plan.name || plan.id}
              <span className="sale-job-meta">SKU {plan.skuId || '-'} · ${plan.quota ?? '-'} · {slotName(plan.slot)}</span>
            </span>
            <span className="sale-job-current">状态 <b>{savedEnabled ? '等待时段' : '计划未启用'}</b></span>
          </div>
        )) : <p className="inline-help">暂无卡档配置。</p>}
      </div>
    </div>
  );
}

function SaleCardRestockRow({ job, planName, slotName }: { job: SaleCardRestockJobStatus; planName: string; slotName: string }) {
  const label = STATUS_LABELS[job.status] ?? job.status;
  const reason = job.failureReason || job.lastError || '';
  return (
    <div className="sale-job-row">
      <span className="sale-job-name">
        {planName}
        <span className="sale-job-meta">
          {job.bizDate || '-'} · {slotName} {job.slotTime || ''} · 尝试 {job.attempts ?? 0} 次 · 超时 {job.consecutiveTimeouts ?? 0} 次
        </span>
        {reason ? <span className="sale-job-meta">原因：{reason}</span> : null}
      </span>
      <span className="sale-job-current">库存 <b>{job.currentStock ?? 0} / {job.targetStock ?? '-'}</b></span>
      <span className="sale-job-current">状态 <b>{label}</b></span>
    </div>
  );
}
