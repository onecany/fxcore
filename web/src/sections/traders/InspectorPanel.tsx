// 编队页详情速览：选中交易员的配置摘要 + 指标 + 打开终端（→ dashboard 联动）。
import { Panel, Badge } from '../../components/ui';
import { ExchangeIcon } from '../../components/exchange-icon';
import type { TraderResponse } from '../../api/v1/types/contract';
import { fmtUsd, fmtPnL, fmtPct } from '../../utils/format';
import { CfgRow } from '../dashboard/primitives';

export function InspectorPanel({ trader, modelNameOf, accountOf, onOpenTerminal }: {
  trader: TraderResponse | null;
  modelNameOf: (t: TraderResponse) => string;
  accountOf: (t: TraderResponse) => string;
  onOpenTerminal: (t: TraderResponse) => void;
}) {
  if (!trader) {
    return (
      <Panel title="DETAILS">
        <div className="muted mono" style={{ fontSize: 11, padding: '8px 0' }}>// SELECT A TRADER</div>
      </Panel>
    );
  }
  const pnl = trader.metrics?.totalPnl ?? 0;
  return (
    <Panel title={`DETAILS · ${trader.name}`}>
      <div className="row wrap insp-head">
        <Badge state={trader.status} />
        <ExchangeIcon type={trader.exchange} size={16} />
        <span className="mono dim">{trader.exchange.toUpperCase()} - {accountOf(trader)}</span>
      </div>
      <div className="tm-grid-5 insp-metrics">
        <InspectorMetric label="PnL" value={fmtPnL(pnl)} tone={pnl >= 0 ? 'up' : 'down'} />
        <InspectorMetric label="WR" value={fmtPct(trader.metrics?.winRate, 0)} />
        <InspectorMetric label="trades" value={String(trader.metrics?.tradeCount ?? 0)} />
        <InspectorMetric label="daily" value={fmtPnL(trader.metrics?.dailyPnl)} tone={(trader.metrics?.dailyPnl ?? 0) >= 0 ? 'up' : 'down'} />
        <InspectorMetric label="ID" value={trader.id.slice(0, 8)} />
      </div>
      <CfgRow k="模型" v={modelNameOf(trader)} />
      <CfgRow k="策略" v={trader.strategyId ?? '-'} />
      <CfgRow k="单笔上限" v={`${fmtUsd(trader.riskConfig?.maxPositionSize)} USDT`} />
      <CfgRow k="止损" v={fmtPct(trader.riskConfig?.stopLoss)} />
      <CfgRow k="止盈" v={fmtPct(trader.riskConfig?.takeProfit)} />
      <CfgRow k="日损上限" v={fmtPct(trader.riskConfig?.maxDailyLoss)} />
      <CfgRow k="周期" v={formatCycle(trader.schedule?.interval)} />
      <button className="btn primary insp-cta" onClick={() => onOpenTerminal(trader)}>
        ▸ 打开终端
      </button>
    </Panel>
  );
}

function InspectorMetric({ label, value, tone }: { label: string; value: string; tone?: 'up' | 'down' }) {
  return (
    <div className="tm-card">
      <div className="tm-card-label">{label}</div>
      <div className={`tm-card-value ${tone ?? ''}`} style={{ fontSize: 15 }}>{value}</div>
    </div>
  );
}

// 周期展示：后端 interval 为秒，UI 单位用分钟（用户偏好）；<60s 显示秒。
function formatCycle(intervalSec?: number): string {
  if (!intervalSec || intervalSec <= 0) return '-';
  if (intervalSec < 60) return `${intervalSec} 秒`;
  const min = intervalSec / 60;
  return Number.isInteger(min) ? `${min} 分钟` : `${min.toFixed(1)} 分钟`;
}
