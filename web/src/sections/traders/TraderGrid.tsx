// 编队网格：trader 卡片（状态灯 + 名称 + meta + 指标行 PnL/WR/trades + 动作按钮），点击选中。
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
        <div className="muted mono" style={{ fontSize: 12, padding: '18px 0', textAlign: 'center' }}>// NO TRADERS IN FLEET</div>
      ) : (
        <div className="fleet-cards">
          {items.map((t) => {
            const selected = t.id === selectedId;
            const pnl = t.metrics?.totalPnl ?? 0;
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
                </div>
                <div className="trader-card-actions" onClick={(e) => e.stopPropagation()}>
                  <button className="btn ghost" onClick={() => onView(t)} style={{ padding: '3px 10px', fontSize: 11 }}>View</button>
                  <button className="btn ghost" onClick={() => onEdit(t)} disabled={t.status === 'running'} style={{ padding: '3px 10px', fontSize: 11 }}>Edit</button>
                  {t.status === 'idle' || t.status === 'stopped' ? (
                    <button className="btn primary" onClick={() => void runAction(t.id, 'start')} style={{ padding: '3px 10px', fontSize: 11 }}>▶ Start</button>
                  ) : t.status === 'running' ? (
                    <>
                      <button className="btn" onClick={() => void runAction(t.id, 'pause')} style={{ padding: '3px 10px', fontSize: 11 }}>⏸ Pause</button>
                      <button className="btn danger" onClick={() => void runAction(t.id, 'stop')} style={{ padding: '3px 10px', fontSize: 11 }}>■ Stop</button>
                    </>
                  ) : (
                    <>
                      <button className="btn primary" onClick={() => void runAction(t.id, 'resume')} style={{ padding: '3px 10px', fontSize: 11 }}>▶ Resume</button>
                      <button className="btn danger" onClick={() => void runAction(t.id, 'stop')} style={{ padding: '3px 10px', fontSize: 11 }}>■ Stop</button>
                    </>
                  )}
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
