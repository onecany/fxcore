// 编队网格：trader 卡片（状态灯 + 名称 + meta + 4 指标格 + 分组动作按钮），点击选中。
// 卡片毛玻璃 + 选中金色光晕；动作按钮分次操作组（View/Edit）与主操作组（Start/Pause/Resume/Stop）。
import { Panel, Badge } from '../../components/ui';
import { ExchangeIcon } from '../../components/exchange-icon';
import type { TraderResponse } from '../../api/v1/types/contract';
import { fmtPnL, fmtPct } from '../../utils/format';
import type { TraderAction } from '../../hooks/useTrader';

export function TraderGrid({ items, selectedId, onSelect, modelNameOf, accountOf, runAction, onEdit, onView }: {
  items: TraderResponse[];
  selectedId: string | null;
  onSelect: (id: string) => void;
  modelNameOf: (t: TraderResponse) => string;
  accountOf: (t: TraderResponse) => string;
  runAction: (id: string, a: TraderAction) => Promise<void>;
  onEdit: (t: TraderResponse) => void;
  onView: (t: TraderResponse) => void;
}) {
  return (
    <Panel title={`Current Traders (${items.length})`} className="fleet-grid-panel">
      {items.length === 0 ? (
        <div className="muted mono empty-hint centered">// NO TRADERS IN FLEET</div>
      ) : (
        <div className="fleet-cards">
          {items.map((t) => {
            const selected = t.id === selectedId;
            const pnl = t.metrics?.totalPnl ?? 0;
            const daily = t.metrics?.dailyPnl ?? 0;
            return (
              <div
                key={t.id}
                className={`trader-card ${selected ? 'selected' : ''}`}
                onClick={() => onSelect(t.id)}
              >
                <div className="trader-card-head">
                  <Badge state={t.status} />
                  <div className="trader-info">
                    <span className="trader-name">{t.name}</span>
                    <span className="trader-meta">
                      <ExchangeIcon type={t.exchange} size={13} />
                      {modelNameOf(t)} · {t.exchange.toUpperCase()} - {accountOf(t)}
                    </span>
                  </div>
                </div>
                <div className="trader-card-metrics">
                  <MetricCell label="PnL" value={fmtPnL(pnl)} tone={pnl >= 0 ? 'up' : 'down'} />
                  <MetricCell label="WR" value={fmtPct(t.metrics?.winRate, 0)} />
                  <MetricCell label="trades" value={String(t.metrics?.tradeCount ?? 0)} />
                  <MetricCell label="daily" value={fmtPnL(daily)} tone={daily >= 0 ? 'up' : 'down'} />
                </div>
                <div className="trader-card-actions" onClick={(e) => e.stopPropagation()}>
                  <div className="tc-group tc-sub">
                    <button className="btn ghost btn-sm" onClick={() => onView(t)}>View</button>
                    <button className="btn ghost btn-sm" onClick={() => onEdit(t)} disabled={t.status === 'running'}>Edit</button>
                  </div>
                  <div className="tc-group tc-main">
                    {t.status === 'idle' || t.status === 'stopped' ? (
                      <button className="btn primary btn-sm" onClick={() => void runAction(t.id, 'start')}>▶ Start</button>
                    ) : t.status === 'running' ? (
                      <>
                        <button className="btn btn-sm" onClick={() => void runAction(t.id, 'pause')}>⏸ Pause</button>
                        <button className="btn danger btn-sm" onClick={() => void runAction(t.id, 'stop')}>■ Stop</button>
                      </>
                    ) : (
                      <>
                        <button className="btn primary btn-sm" onClick={() => void runAction(t.id, 'resume')}>▶ Resume</button>
                        <button className="btn danger btn-sm" onClick={() => void runAction(t.id, 'stop')}>■ Stop</button>
                      </>
                    )}
                  </div>
                </div>
              </div>
            );
          })}
        </div>
      )}
    </Panel>
  );
}

function MetricCell({ label, value, tone }: { label: string; value: string; tone?: 'up' | 'down' }) {
  return (
    <div className="metric-cell">
      <span className="metric-label">{label}</span>
      <span className={`metric-value ${tone ?? ''}`}>{value}</span>
    </div>
  );
}
