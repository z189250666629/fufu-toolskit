import { MessageLine } from '../blueprint';
import { Metric } from './adminShared';
import { SALE_SLOTS } from './saleCardConfigCore';
import type { SaleCardConfig } from './types';

const PAUSED_MESSAGE = '自动补卡和 MCY 库存检测已暂时下线，当前不查询库存，也不对接商城上架。';

export function SaleCardManager({ config }: { config?: SaleCardConfig }) {
  const plans = config?.plans ?? [];
  const savedSchedule = config?.schedule;
  const savedEnabled = Boolean(savedSchedule?.enabled || savedSchedule?.slots?.some((slot) => slot.enabled));

  const slotName = (group?: string) => SALE_SLOTS.find((slot) => slot.group === group)?.label ?? group ?? '-';

  return (
    <div className="business-stack">
      <MessageLine tone="error">{PAUSED_MESSAGE}</MessageLine>
      <p className="inline-help">保留卡档数据用于活动配置与测试 key；后台不会启动自动补卡任务，页面也不会触发 MCY 库存检测。</p>
      <div className="metrics sale-run-result">
        <Metric label="卡档数量" value={plans.length} />
        <Metric label="自动计划" value={savedEnabled ? '已保存但不运行' : '未启用'} />
      </div>
      <div className="config-subhead">卡档参考（不查询 MCY 库存）</div>
      <div className="sale-job-list">
        {plans.length ? plans.map((plan) => (
          <div className="sale-job-row" key={plan.id}>
            <span className="sale-job-name">
              {plan.name || plan.id}
              <span className="sale-job-meta">SKU {plan.skuId || '-'} · ${plan.quota ?? '-'} · {slotName(plan.slot)}</span>
            </span>
            <span className="sale-job-current">状态 <b>未对接</b></span>
          </div>
        )) : <p className="inline-help">暂无卡档配置。</p>}
      </div>
    </div>
  );
}
