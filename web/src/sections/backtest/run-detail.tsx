// Backtest 运行详情视图：权益曲线 + 绩效指标 + 成交明细 + 决策流（分页）+ 单周期 trace。
// 数据层 SWR keyed by runId；run 活跃（running/created）时 3s 轮询，静止后停轮。
// 契约说明：/backtest/metrics 未就绪返回 HTTP 202 + data:null（client 解信封后为 null）；
// /backtest/decisions 返回分页信封 { items, pagination }（client 已转 camelCase）。
import { useCallback, useMemo, useState } from 'react';
import useSWR from 'swr';
import * as backtestApi from '../../api/v1/modules/backtest';
import type {
  RunSummary,
  BacktestEquityPoint,
  BacktestTradeEvent,
  BacktestMetrics,
  DecisionRecord,
  EquitySnapshot,
} from '../../api/v1/types/contract';
import { Badge, Panel, Alert } from '../../components/ui';
import { EquityCurve, Pager } from '../dashboard/primitives';
import { DecisionCard } from '../dashboard/exec-log';
import { fmtUsd, fmtPnL, fmtPct, fmtTime } from '../../utils/format';
import { errMsg } from '../../utils/errorCodes';

const ACTIVE_STATES = new Set(['running', 'created']);

/** 权益点 → EquitySnapshot（EquityCurve 复用映射） */
function toEquitySnapshots(points: BacktestEquityPoint[]): EquitySnapshot[] {
  return points
    .map((p) => ({
      timestamp: p.timestamp,
      equity: p.equity,
      balance: p.available,
      pnl: p.pnl,
      drawdownPct: p.drawdownPct,
    }))
    .sort((a, b) => a.timestamp - b.timestamp); // 数据源可能降序，曲线必须升序
}

const TRADE_ACTION_LABEL: Record<string, string> = {
  open: '开仓',
  close: '平仓',
  liquidate: '强平',
};

/** 后端口径 0-100 数字（非小数）→ 百分比文案：8.4 → 8.4% */
function pctNum(v: number | undefined | null, precision = 1): string {
  if (typeof v !== 'number' || Number.isNaN(v)) return '-';
  return `${v.toFixed(precision)}%`;
}

export default function RunDetail({
  run,
  onClose,
  onRemoved,
}: {
  run: RunSummary;
  onClose: () => void;
  onRemoved: () => void;
}) {
  const runId = run.runId;
  const active = ACTIVE_STATES.has(run.state);

  // ===== 权益曲线（limit 400 抽稀对齐 dashboard 实践） =====
  const { data: equityData } = useSWR<BacktestEquityPoint[]>(
    ['/backtest/equity', runId],
    () => backtestApi.backtestEquity(runId, undefined, 400),
    { refreshInterval: active ? 3000 : 0 },
  );
  const equityPoints = useMemo(() => toEquitySnapshots(equityData ?? []), [equityData]);

  // ===== 绩效指标（未就绪 → null） =====
  const { data: metrics, error: metricsErr } = useSWR<BacktestMetrics | null>(
    ['/backtest/metrics', runId],
    () => backtestApi.backtestMetrics(runId),
    { refreshInterval: active ? 3000 : 0 },
  );

  // ===== 成交明细 =====
  const { data: trades } = useSWR<BacktestTradeEvent[]>(
    ['/backtest/trades', runId],
    () => backtestApi.backtestTrades(runId),
    { refreshInterval: active ? 3000 : 0 },
  );
  const tradeList = useMemo(() => trades ?? [], [trades]);

  // ===== 决策流（分页） =====
  const [page, setPage] = useState(1);
  const { data: decisionsData } = useSWR(
    ['/backtest/decisions', runId, page],
    () => backtestApi.backtestDecisions(runId, page, 10),
    { refreshInterval: active ? 3000 : 0 },
  );
  const decisions = useMemo(() => decisionsData?.items ?? [], [decisionsData]);
  const totalPages = useMemo(() => Math.max(1, decisionsData?.pagination?.totalPages ?? 1), [decisionsData]);

  // ===== 单周期 trace（点击决策行展开） =====
  const [traceCycle, setTraceCycle] = useState<number | null>(null);
  const { data: trace, error: traceErr } = useSWR<DecisionRecord | null>(
    ['/backtest/trace', runId, traceCycle],
    () => (traceCycle != null ? backtestApi.backtestTrace(runId, traceCycle) : Promise.resolve(null)),
    { revalidateOnFocus: false },
  );

  const exportZip = useCallback(async () => {
    try {
      const blob = await backtestApi.exportBacktest(runId);
      const url = URL.createObjectURL(blob);
      const a = document.createElement('a');
      a.href = url;
      a.download = `backtest-${runId}.zip`;
      document.body.appendChild(a);
      a.click();
      a.remove();
      URL.revokeObjectURL(url);
    } catch (err) {
      alert(`导出失败：${errMsg(String(err))}`);
    }
  }, [runId]);

  const toggleTrace = useCallback(
    (cycle: number) => setTraceCycle((cur) => (cur === cycle ? null : cycle)),
    [],
  );

  return (
    <div className="bt-detail">
      <div className="bt-detail-head">
        <div className="bt-detail-id">
          <span className="bt-detail-label">{run.label || '回测运行'}</span>
          <span className="mono dim bt-detail-runid">{runId}</span>
        </div>
        <Badge state={run.state} />
        <div className="bt-detail-actions">
          <button className="btn ghost btn-sm" onClick={() => void exportZip()}>⇩ 导出 zip</button>
          <button className="btn btn-sm" onClick={onClose}>✕ 关闭</button>
        </div>
      </div>

      <Panel title="权益曲线" className="mb">
        <EquityCurve points={equityPoints} height={150} />
        <div className="mono dim bt-detail-subline">
          {equityPoints.length > 0
            ? `${equityPoints.length} 个权益点 · ${fmtUsd(equityPoints[0].equity)} → ${fmtUsd(equityPoints[equityPoints.length - 1].equity)}`
            : '// 暂无权益数据'}
        </div>
      </Panel>

      <Panel title="绩效指标" className="mb">
        {metricsErr ? (
          <Alert kind="error">{errMsg(String(metricsErr))}</Alert>
        ) : metrics ? (
          <div className="bt-metrics-grid">
            <Metric k="总交易" v={String(metrics.totalTrades ?? 0)} />
            <Metric k="胜率" v={fmtPct(metrics.winRate)} />
            <Metric k="总盈亏" v={fmtPnL(metrics.totalPnl)} tone={(metrics.totalPnl ?? 0) >= 0 ? 'up' : 'down'} />
            <Metric k="盈亏比" v={metrics.profitFactor != null ? metrics.profitFactor.toFixed(2) : '—'} />
            <Metric k="夏普" v={metrics.sharpeRatio != null ? metrics.sharpeRatio.toFixed(2) : '—'} />
            {/* 后端口径：maxDrawdownPct/returnPct 为 0-100 数字（非小数），winRate 为 0-1 */}
            <Metric k="最大回撤" v={pctNum(metrics.maxDrawdownPct)} tone={(metrics.maxDrawdownPct ?? 0) > 0 ? 'down' : undefined} />
            <Metric k="平均盈利" v={fmtUsd(metrics.avgWin)} tone="up" />
            <Metric k="平均亏损" v={fmtUsd(metrics.avgLoss)} tone="down" />
            <Metric k="期末权益" v={fmtUsd(metrics.finalEquity)} />
            <Metric k="收益率" v={pctNum(metrics.returnPct, 2)} tone={(metrics.returnPct ?? 0) >= 0 ? 'up' : 'down'} />
            {metrics.liquidated && <Metric k="状态" v="爆仓" tone="down" />}
          </div>
        ) : (
          <div className="muted mono bt-detail-pending">// 回放进行中，指标待计算</div>
        )}
      </Panel>

      <Panel title={`成交明细 (${tradeList.length})`} className="mb">
        {tradeList.length === 0 ? (
          <div className="muted mono bt-detail-pending">// 暂无成交</div>
        ) : (
          <div className="table-wrap">
            <table className="bt-detail-table">
              <thead>
                <tr>
                  <th>时间</th>
                  <th>币种</th>
                  <th>动作</th>
                  <th>方向</th>
                  <th>数量</th>
                  <th>价格</th>
                  <th>手续费</th>
                  <th>滑点</th>
                  <th>订单额</th>
                  <th>已实现盈亏</th>
                  <th>杠杆</th>
                  <th>持仓后</th>
                </tr>
              </thead>
              <tbody>
                {tradeList.map((t, i) => (
                  <tr key={`${t.timestamp}-${i}`}>
                    <td className="mono">{fmtTime(t.timestamp)}</td>
                    <td>{t.symbol}</td>
                    <td>{TRADE_ACTION_LABEL[t.action] ?? t.action}</td>
                    <td className="mono">{t.side ?? '—'}</td>
                    <td className="mono">{t.quantity}</td>
                    <td className="mono">{fmtUsd(t.price)}</td>
                    <td className="mono">{t.fee != null ? t.fee.toFixed(4) : '—'}</td>
                    <td className="mono">{t.slippage != null ? t.slippage.toFixed(4) : '—'}</td>
                    <td className="mono">{fmtUsd(t.orderValue)}</td>
                    <td className="mono" style={{ color: t.realizedPnl >= 0 ? 'var(--fxcore-up)' : 'var(--fxcore-down)' }}>
                      {fmtPnL(t.realizedPnl)}
                    </td>
                    <td className="mono">{t.leverage != null ? `${t.leverage}x` : '—'}</td>
                    <td className="mono">{t.positionAfter != null ? t.positionAfter.toFixed(4) : '—'}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </Panel>

      <Panel title={`决策流 · 第 ${page} 页`} className="mb">
        {decisions.length === 0 ? (
          <div className="muted mono bt-detail-pending">// 暂无决策记录</div>
        ) : (
          <>
            <div className="bt-decisions">
              {decisions.map((d) => (
                <DecisionRow key={d.cycleNumber} d={d} open={traceCycle === d.cycleNumber} onToggle={toggleTrace} />
              ))}
            </div>
            <Pager page={page} totalPages={totalPages} onPage={setPage} />
          </>
        )}
      </Panel>

      {traceErr && <Alert kind="error">trace 加载失败：{errMsg(String(traceErr))}</Alert>}
      {traceCycle != null && trace && (
        <Panel title={`单周期 trace · Cycle #${traceCycle}`} className="mb">
          <DecisionCard d={trace} />
          <button className="btn ghost btn-sm" style={{ marginTop: 8 }} onClick={() => setTraceCycle(null)}>
            ✕ 收起 trace
          </button>
        </Panel>
      )}

      <div className="bt-detail-foot">
        <button className="btn danger btn-sm" onClick={onRemoved}>删除该回测（含关联数据）</button>
      </div>
    </div>
  );
}

function Metric({ k, v, tone }: { k: string; v: string; tone?: 'up' | 'down' }) {
  return (
    <div className="bt-metric">
      <span className="bt-metric-label">{k}</span>
      <span className="bt-metric-value mono" style={tone === 'up' ? { color: 'var(--fxcore-up)' } : tone === 'down' ? { color: 'var(--fxcore-down)' } : undefined}>
        {v}
      </span>
    </div>
  );
}

function DecisionRow({ d, open, onToggle }: { d: DecisionRecord; open: boolean; onToggle: (c: number) => void }) {
  const summary = useMemo(() => {
    const acts = d.decisions ?? [];
    if (acts.length === 0) return '—';
    return acts.map((a) => `${a.symbol ?? 'ALL'} ${a.action}`).join(' · ');
  }, [d]);
  return (
    <div className={`bt-decision-row ${open ? 'open' : ''}`}>
      <button className="bt-decision-head" onClick={() => onToggle(d.cycleNumber)}>
        <span className="mono bt-decision-cycle">Cycle #{d.cycleNumber}</span>
        <span className="mono dim">{fmtTime(d.timestamp)}</span>
        <span className="dim bt-decision-summary">{summary}</span>
        <span className="mono dim" style={{ marginLeft: 'auto' }}>{open ? '▾' : '▸'}</span>
      </button>
    </div>
  );
}
