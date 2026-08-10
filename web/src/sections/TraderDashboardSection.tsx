// 交易员详情仪表盘（指挥中心信息架构）：
// 顶栏：交易员下拉 + ID 截断 + tm 终端摘要行（model/strategy/lev/positions/next cycle）
// → tm 指标网格（Equity/Total P&L/Realized/Profit Factor/Max DD）+ Risk Radar 风险雷达
// → 权益曲线（Initial/Current/Cycles 标注）
// → Current Positions + Execution Log（决策时间线，可展开 Prompt / Chain of Thought）
// → Position History（统计网格 + LONG/SHORT 分拆 + Symbol Performance + 明细表）
// 支持 URL ?trader=<id> 直达选中。
import { useCallback, useEffect, useMemo, useState } from 'react';
import * as traderApi from '../api/v1/modules/traders';
import * as dataApi from '../api/v1/modules/data';
import type { TraderResponse, DecisionRecord, EquitySnapshot, Position, Order } from '../api/v1/types/contract';
import { PageHead, Panel, Badge } from '../components/ui';
import { fmtUsd, fmtPct, fmtDur } from '../utils/format';

const UP = 'var(--fxcore-up)';
const DOWN = 'var(--fxcore-down)';

function fmtTime(raw?: string | number): string {
  if (!raw) return '-';
  const d = typeof raw === 'number' ? new Date(raw * 1000) : new Date(raw);
  return d.toLocaleTimeString('zh-CN', { hour12: false });
}
function fmtClock(raw?: string): string {
  if (!raw) return '-';
  const d = new Date(raw);
  return d.toLocaleTimeString('zh-CN', { hour12: false });
}
function pnlColor(v: number | undefined | null): string {
  return (v ?? 0) >= 0 ? UP : DOWN;
}
/** 平仓盈亏双拼写兼容：/positions/history 裸 model 直出 realizedPnl（小写 l），DTO 风格 realizedPnL（大写 L） */
function realizedOf(p: Position): number | undefined {
  return p.realizedPnL ?? p.realizedPnl;
}

/** 权益曲线 SVG（迷你 sparkline） */
function EquityCurve({ points, height = 130 }: { points: EquitySnapshot[]; height?: number }) {
  const path = useMemo(() => {
    if (points.length < 2) return null;
    const w = 560;
    const h = height;
    const vals = points.map((p) => p.equity);
    const min = Math.min(...vals);
    const max = Math.max(...vals);
    const range = max - min || 1;
    const step = w / (points.length - 1);
    const coords = points.map((p, i) => [i * step, h - ((p.equity - min) / range) * (h - 10) - 5] as const);
    const d = coords.map(([x, y], i) => `${i === 0 ? 'M' : 'L'}${x.toFixed(1)},${y.toFixed(1)}`).join(' ');
    return { d, min, max };
  }, [points, height]);
  if (!path) return <div className="muted mono" style={{ fontSize: 12 }}>// 暂无权益数据</div>;
  const up = points[points.length - 1].equity >= points[0].equity;
  return (
    <svg viewBox={`0 0 560 ${height}`} style={{ width: '100%', height, display: 'block' }}>
      <defs>
        <linearGradient id="eqfill" x1="0" y1="0" x2="0" y2="1">
          <stop offset="0%" stopColor="var(--fxcore-accent)" stopOpacity="0.25" />
          <stop offset="100%" stopColor="var(--fxcore-accent)" stopOpacity="0.02" />
        </linearGradient>
      </defs>
      {[0.25, 0.5, 0.75].map((f) => (
        <line key={f} x1="0" x2="560" y1={height * f} y2={height * f} stroke="var(--fxcore-panel-border)" strokeWidth="0.5" />
      ))}
      <path d={`${path.d} L560,${height} L0,${height} Z`} fill="url(#eqfill)" />
      <path d={path.d} fill="none" stroke={up ? 'var(--fxcore-up)' : 'var(--fxcore-down)'} strokeWidth="1.6" />
    </svg>
  );
}

export default function TraderDashboardSection() {
  const [traders, setTraders] = useState<TraderResponse[]>([]);
  const [traderId, setTraderId] = useState<string>('');
  const [equities, setEquities] = useState<EquitySnapshot[]>([]);
  const [decisions, setDecisions] = useState<DecisionRecord[]>([]);
  const [positions, setPositions] = useState<Position[]>([]);
  const [history, setHistory] = useState<Position[]>([]);
  const [orders, setOrders] = useState<Order[]>([]);
  const [error, setError] = useState<string | null>(null);

  const loadTraders = useCallback(async () => {
    try {
      const d = await traderApi.listTraders({ size: 50 });
      setTraders(d.items);
      // URL ?trader=<id> 优先，否则首个 running，否则第一个
      const q = new URLSearchParams(window.location.search).get('trader');
      const preferred = q ? d.items.find((t) => t.id === q) : d.items.find((t) => t.status === 'running');
      setTraderId((cur) => cur || preferred?.id || d.items[0]?.id || '');
      setError(null);
    } catch (e) { setError(String(e)); }
  }, []);

  useEffect(() => { void loadTraders(); }, [loadTraders]);

  // trader 变化时拉全部详情数据
  useEffect(() => {
    if (!traderId) return;
    let alive = true;
    (async () => {
      try {
        const [eq, dc, ps, hs, od] = await Promise.all([
          dataApi.equityHistory(traderId),
          dataApi.listDecisions({ trader_id: traderId, size: 8 }),
          traderApi.listPositions({}),
          dataApi.positionHistory({ trader_id: traderId, limit: 50 }),
          dataApi.listOrders({ trader_id: traderId }),
        ]);
        if (!alive) return;
        setEquities(eq ?? []);
        setDecisions((dc.items ?? []).filter((d) => d.traderId === traderId));
        setPositions((ps ?? []).filter((p) => p.traderId === traderId && !p.closedAt));
        setHistory((hs.items ?? []).filter((p) => p.traderId === traderId));
        setOrders((od.items ?? []).filter((o) => o.traderId === traderId));
        setError(null);
      } catch (e) { if (alive) setError(String(e)); }
    })();
    return () => { alive = false; };
  }, [traderId]);

  const trader = useMemo(() => traders.find((t) => t.id === traderId), [traders, traderId]);
  const latest = decisions[0];
  const equity = equities[equities.length - 1];
  const pnl = trader?.metrics?.totalPnl ?? 0;
  const maxDd = useMemo(() => {
    if (equities.length < 2) return 0;
    let peak = equities[0].equity;
    let dd = 0;
    for (const p of equities) {
      if (p.equity > peak) peak = p.equity;
      if (peak > 0) dd = Math.max(dd, (peak - p.equity) / peak);
    }
    return dd;
  }, [equities]);

  // 权益变化率（相对首快照）
  const equityChgPct = useMemo(() => {
    if (equities.length < 2 || equities[0].equity <= 0) return 0;
    return (equities[equities.length - 1].equity - equities[0].equity) / equities[0].equity;
  }, [equities]);

  // Position History 统计（从平仓历史聚合）
  const stats = useMemo(() => {
    const closed = history;
    const total = closed.length;
    const wins = closed.filter((p) => (realizedOf(p) ?? 0) > 0);
    const losses = closed.filter((p) => (realizedOf(p) ?? 0) <= 0);
    const totalPnl = closed.reduce((s, p) => s + (realizedOf(p) ?? 0), 0);
    const fee = closed.reduce((s, p) => s + (p.fee ?? 0), 0);
    const winSum = wins.reduce((s, p) => s + (realizedOf(p) ?? 0), 0);
    const lossSum = Math.abs(losses.reduce((s, p) => s + (realizedOf(p) ?? 0), 0));
    const long = closed.filter((p) => p.side === 'long');
    const short = closed.filter((p) => p.side === 'short');
    const bySymbol = new Map<string, { trades: number; wins: number; pnl: number }>();
    for (const p of closed) {
      const e = bySymbol.get(p.symbol) ?? { trades: 0, wins: 0, pnl: 0 };
      e.trades += 1;
      if ((realizedOf(p) ?? 0) > 0) e.wins += 1;
      e.pnl += (realizedOf(p) ?? 0);
      bySymbol.set(p.symbol, e);
    }
    const winRate = total > 0 ? wins.length / total : 0;
    return {
      total,
      winRate,
      totalPnl,
      fee,
      profitFactor: lossSum > 0 ? winSum / lossSum : (winSum > 0 ? Infinity : 0),
      avgWin: wins.length > 0 ? winSum / wins.length : 0,
      avgLoss: losses.length > 0 ? -(lossSum / losses.length) : 0,
      netPnl: totalPnl - fee,
      long: { count: long.length, winRate: long.length > 0 ? long.filter((p) => (realizedOf(p) ?? 0) > 0).length / long.length : 0, pnl: long.reduce((s, p) => s + (realizedOf(p) ?? 0), 0) },
      short: { count: short.length, winRate: short.length > 0 ? short.filter((p) => (realizedOf(p) ?? 0) > 0).length / short.length : 0, pnl: short.reduce((s, p) => s + (realizedOf(p) ?? 0), 0) },
      bySymbol: [...bySymbol.entries()].sort((a, b) => b[1].pnl - a[1].pnl),
    };
  }, [history]);

  return (
    <section>
      <PageHead
        title="交易终端"
        lead={<>交易员详情 · URL <code>?trader=</code> 直达</>}
      />
      {error && <div className="alert error">{error}</div>}

      {/* 顶栏：orchestration select + 名称 ID + 状态 */}
      <div className="row wrap" style={{ gap: 12, marginBottom: 14 }}>
        <label className="dim" style={{ fontSize: 12 }}>orchestration</label>
        <select
          value={traderId}
          onChange={(e) => { setTraderId(e.target.value); window.history.replaceState(null, '', `?trader=${e.target.value}`); }}
          className="mono"
          style={{ background: 'var(--fxcore-panel)', color: 'var(--fxcore-text)', border: '1px solid var(--fxcore-panel-border)', padding: '4px 8px', fontSize: 12 }}
        >
          {traders.map((t) => (
            <option key={t.id} value={t.id}>{t.name} · {t.exchange}</option>
          ))}
        </select>
        <Badge state={trader?.status ?? 'idle'} />
        {trader && (
          <span className="mono dim" style={{ fontSize: 12 }}>
            ID: {trader.id.slice(0, 8)}...
          </span>
        )}
        <span className="mono dim" style={{ fontSize: 12 }}>
          cycle <b style={{ color: 'var(--fxcore-accent)' }}>{latest?.cycleNumber ?? '-'}</b>
        </span>
      </div>

      {/* tm 终端摘要行（name · model · strategy · lev · scan · universe · positions · next cycle） */}
      <div className="tm-mono" style={{ marginBottom: 12 }}>
        <span className="tm-k">{trader?.name ?? '-'}</span>
        <span className="tm-dim">model</span><span className="tm-v">{trader?.modelConfig?.modelId ?? '-'}</span>
        <span className="tm-dim">strategy</span><span className="tm-v">{trader?.strategyId?.slice(0, 8) ?? '-'}</span>
        <span className="tm-dim">lev</span><span className="tm-v">—× / —×</span>
        <span className="tm-dim">positions</span><span className="tm-v">{positions.length}</span>
        <span className="tm-dim">cycle</span><span className="tm-v">{latest?.cycleNumber ?? '-'}</span>
        <span className="tm-dim">status</span>
        <span className={`tm-v ${trader?.status === 'running' ? 'ok' : trader?.status === 'paused' ? 'warn' : ''}`}>
          {trader?.status === 'running' ? 'ONLINE' : (trader?.status ?? 'OFFLINE').toUpperCase()}
        </span>
        <span className="tm-dim">eq</span><span className="tm-v">{fmtUsd(equity?.equity)}</span>
        <span className="tm-dim">pnl</span><span className="tm-v" style={{ color: pnlColor(pnl) }}>{fmtUsd(pnl, true)}</span>
      </div>

      {/* tm 指标网格：Equity / Total P&L(含未实现) / Realized / Profit Factor / Max DD */}
      <div className="tm-grid-5" style={{ marginBottom: 12 }}>
        <TmCard label="EQUITY" value={fmtUsd(equity?.equity)} sub={`${fmtPct(equityChgPct)} ${equityChgPct >= 0 ? '▲' : '▼'}`} color={equityChgPct >= 0 ? UP : DOWN} />
        <TmCard label="TOTAL P&L · INCL. UNREALIZED" value={fmtUsd(pnl, true)} sub={fmtPct(equity?.equity ? pnl / equity.equity : 0)} color={pnlColor(pnl)} />
        <TmCard label="TOTAL P&L · CLOSED" value={fmtUsd(stats.totalPnl, true)} color={pnlColor(stats.totalPnl)} />
        <TmCard label="PROFIT FACTOR" value={Number.isFinite(stats.profitFactor) ? stats.profitFactor.toFixed(2) : '-'} sub="总盈/总亏" />
        <TmCard label="MAX DRAWDOWN" value={fmtPct(maxDd)} color={maxDd > 0.1 ? DOWN : undefined} sub="自峰回撤" />
      </div>

      {/* tm 第二行指标：trades / win / loss / net / sharpe / avg win-loss */}
      <div className="tm-mono tm-line2" style={{ marginBottom: 14 }}>
        <span className="tm-dim">trades</span><span className="tm-v">{stats.total}</span>
        <span className="tm-dim">win</span><span className="tm-v ok">{history.filter((p) => (realizedOf(p) ?? 0) > 0).length} ({fmtPct(stats.winRate)})</span>
        <span className="tm-dim">loss</span><span className="tm-v down">{history.filter((p) => (realizedOf(p) ?? 0) <= 0).length}</span>
        <span className="tm-dim">net</span><span className="tm-v" style={{ color: pnlColor(stats.netPnl) }}>{fmtUsd(stats.netPnl, true)}</span>
        <span className="tm-dim">fees</span><span className="tm-v">{fmtUsd(stats.fee)}</span>
        <span className="tm-dim">sharpe/trade</span><span className="tm-v">{fmtSharpe(history)}</span>
        <span className="tm-dim">avg win/loss</span><span className="tm-v">{fmtUsd(stats.avgWin, true)} / {fmtUsd(stats.avgLoss, true)}</span>
      </div>

      {/* Risk Radar：风险雷达（从 equity/positions/history 聚合） */}
      <div className="tm-grid-7" style={{ marginBottom: 14 }}>
        <RiskCard label="NET EXPOSURE" value="Flat" sub={`long ${fmtUsd(0)} / short ${fmtUsd(0)}`} />
        <RiskCard label="LEVERAGE" value="—" sub={`${fmtPct(equity?.marginUsedPct)} avg / — peak`} />
        <RiskCard label="MARGIN USED" value={equity?.marginUsedPct != null && equity.marginUsedPct < 0.3 ? 'Ample' : equity?.marginUsedPct != null && equity.marginUsedPct < 0.7 ? 'Moderate' : 'Tight'} sub={`${fmtPct(equity?.marginUsedPct)} of equity`} />
        <RiskCard label="MAX DRAWDOWN" value={maxDd < 0.05 ? 'Calm' : maxDd < 0.15 ? 'Watch' : 'Alert'} sub={`${fmtPct(maxDd)} peak drawdown`} />
        <RiskCard label="POSITIONS" value={`${positions.length}`} sub={`${positions.length} held / cap`} />
        <RiskCard label="UNREALIZED PNL" value={fmtUsd(positions.reduce((s, p) => s + (p.unrealizedPnL ?? 0), 0), true)} color={pnlColor(positions.reduce((s, p) => s + (p.unrealizedPnL ?? 0), 0))} />
        <RiskCard label="AVAILABLE" value={fmtUsd(equity?.balance)} sub={`${fmtPct(equity?.marginUsedPct)} 占用`} />
      </div>

      <div className="td-grid td-grid-3">
        {/* 左列：权益曲线 + 配置摘要 */}
        <div style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
          <Panel title="权益曲线">
            <EquityCurve points={equities} />
            <div className="row wrap" style={{ marginTop: 8, fontSize: 11, color: 'var(--fxcore-text-faint)' }}>
              <span>Initial {fmtUsd(equities[0]?.equity)}</span>
              <span>· Current {fmtUsd(equity?.equity)}</span>
              <span>· Cycles {equities.length}</span>
              <span>· Margin {fmtPct(equity?.marginUsedPct)}</span>
            </div>
          </Panel>
          <Panel title="配置摘要">
            <CfgRow k="交易所" v={trader?.exchange ?? '-'} />
            <CfgRow k="模型" v={`${trader?.modelConfig?.provider ?? '-'} / ${trader?.modelConfig?.modelId ?? '-'}`} />
            <CfgRow k="策略" v={trader?.strategyId ?? '-'} />
            <CfgRow k="单笔上限" v={`${fmtUsd(trader?.riskConfig?.maxPositionSize)} USDT`} />
            <CfgRow k="止损" v={fmtPct(trader?.riskConfig?.stopLoss)} />
            <CfgRow k="止盈" v={fmtPct(trader?.riskConfig?.takeProfit)} />
            <CfgRow k="日损上限" v={fmtPct(trader?.riskConfig?.maxDailyLoss)} />
          </Panel>
        </div>

        {/* 中列：Current Positions + Recent Decisions */}
        <div style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
          <Panel title="Current Positions">
            {positions.length > 0 ? (
              <div className="table-wrap"><table className="data-table">
                <thead><tr><th>币种</th><th>方向</th><th>数量</th><th>开仓价</th><th>标记价</th><th>浮盈</th></tr></thead>
                <tbody>
                  {positions.map((p) => (
                    <tr key={p.id}>
                      <td className="mono">{p.symbol}</td>
                      <td style={{ color: p.side === 'long' ? UP : DOWN }}>{p.side === 'long' ? '多' : '空'}</td>
                      <td className="mono">{p.quantity}</td>
                      <td className="mono">{p.entryPrice}</td>
                      <td className="mono">{p.markPrice ?? '-'}</td>
                      <td className="mono" style={{ color: pnlColor(p.unrealizedPnL) }}>{fmtUsd(p.unrealizedPnL, true)}</td>
                    </tr>
                  ))}
                </tbody>
              </table></div>
            ) : <div className="muted mono" style={{ fontSize: 12 }}>📊 No Positions · 无活动持仓</div>}
          </Panel>
          <Panel title={`Execution Log (${decisions.length} cyc)`}>
            {decisions.length > 0 ? (
              <div style={{ display: 'flex', flexDirection: 'column', gap: 8 }}>
                {decisions.map((d) => <DecisionCard key={d.id} d={d} />)}
              </div>
            ) : <div className="muted mono" style={{ fontSize: 12 }}>// 暂无决策记录</div>}
          </Panel>
        </div>

        {/* 右列：订单 + Position History 统计 */}
        <div style={{ display: 'flex', flexDirection: 'column', gap: 12 }}>
          <Panel title={`订单 (${orders.length})`}>
            {orders.length > 0 ? (
              <div className="table-wrap"><table className="data-table">
                <thead><tr><th>币种</th><th>类型</th><th>状态</th><th>数量</th><th>价格</th></tr></thead>
                <tbody>
                  {orders.slice(0, 6).map((o) => (
                    <tr key={o.id}>
                      <td className="mono">{o.symbol}</td>
                      <td className="mono dim">{o.type}</td>
                      <td><span className="mono" style={{ color: o.status === 'FILLED' ? UP : o.status === 'NEW' ? 'var(--fxcore-accent)' : 'var(--fxcore-text-faint)' }}>{o.status}</span></td>
                      <td className="mono">{o.quantity}</td>
                      <td className="mono">{o.price ?? o.avgFillPrice ?? '-'}</td>
                    </tr>
                  ))}
                </tbody>
              </table></div>
            ) : <div className="muted mono" style={{ fontSize: 12 }}>// NO ORDERS</div>}
          </Panel>
          <Panel title={`Position History (${stats.total})`}>
            {stats.total > 0 ? (
              <>
                <div className="ph-stats">
                  <PhStat label="WIN RATE" value={fmtPct(stats.winRate)} hint={`Win ${history.filter((p) => (realizedOf(p) ?? 0) > 0).length} / Loss ${history.filter((p) => (realizedOf(p) ?? 0) <= 0).length}`} />
                  <PhStat label="NET P&L" value={fmtUsd(stats.netPnl, true)} color={pnlColor(stats.netPnl)} hint={`Fee ${fmtUsd(stats.fee)}`} />
                  <PhStat label="PROFIT FACTOR" value={Number.isFinite(stats.profitFactor) ? stats.profitFactor.toFixed(2) : '-'} hint="总盈/总亏" />
                  <PhStat label="AVG WIN" value={fmtUsd(stats.avgWin, true)} color={UP} />
                  <PhStat label="AVG LOSS" value={fmtUsd(stats.avgLoss, true)} color={DOWN} />
                  <PhStat label="SHARPE" value={fmtSharpe(history)} hint="风险调整收益" />
                </div>
                <div className="row wrap" style={{ marginTop: 10, gap: 6 }}>
                  <span className="chip">{`LONG ${stats.long.count} · WR ${fmtPct(stats.long.winRate)} · ${fmtUsd(stats.long.pnl, true)}`}</span>
                  <span className="chip">{`SHORT ${stats.short.count} · WR ${fmtPct(stats.short.winRate)} · ${fmtUsd(stats.short.pnl, true)}`}</span>
                </div>
                {stats.bySymbol.length > 0 && (
                  <div style={{ marginTop: 10 }}>
                    <div className="mono dim" style={{ fontSize: 10.5, letterSpacing: '0.1em', marginBottom: 4 }}>SYMBOL PERFORMANCE</div>
                    <div className="table-wrap"><table className="data-table">
                      <thead><tr><th>币种</th><th>笔数</th><th>胜率</th><th>P&L</th></tr></thead>
                      <tbody>
                        {stats.bySymbol.slice(0, 6).map(([sym, e]) => (
                          <tr key={sym}>
                            <td className="mono">{sym}</td>
                            <td className="mono">{e.trades}</td>
                            <td className="mono">{e.trades > 0 ? `${((e.wins / e.trades) * 100).toFixed(0)}%` : '-'}</td>
                            <td className="mono" style={{ color: pnlColor(e.pnl) }}>{fmtUsd(e.pnl, true)}</td>
                          </tr>
                        ))}
                      </tbody>
                    </table></div>
                  </div>
                )}
                <div style={{ marginTop: 10 }}>
                  <div className="mono dim" style={{ fontSize: 10.5, letterSpacing: '0.1em', marginBottom: 4 }}>CLOSED TRADES</div>
                  <div className="table-wrap"><table className="data-table">
                    <thead><tr><th>币种</th><th>方向</th><th>盈亏</th><th>Fee</th><th>时长</th><th>平仓</th></tr></thead>
                    <tbody>
                      {history.slice(0, 8).map((p) => (
                        <tr key={p.id}>
                          <td className="mono">{p.symbol}</td>
                          <td style={{ color: p.side === 'long' ? UP : DOWN }}>{p.side === 'long' ? '多' : '空'}</td>
                          <td className="mono" style={{ color: pnlColor(realizedOf(p)) }}>{fmtUsd(realizedOf(p), true)}</td>
                          <td className="mono dim">{fmtUsd(p.fee)}</td>
                          <td className="mono dim">{fmtDur(p.openedAt, p.closedAt)}</td>
                          <td className="mono dim">{p.closedAt ? fmtClock(p.closedAt) : '-'}</td>
                        </tr>
                      ))}
                    </tbody>
                  </table></div>
                </div>
              </>
            ) : <div className="muted mono" style={{ fontSize: 12 }}>// NO CLOSED POSITIONS</div>}
          </Panel>
        </div>
      </div>
    </section>
  );
}

/** 执行日志卡：Cycle 头 + 动作时间线 + AI duration + 结果行 + 可展开 Prompt / COT */
function DecisionCard({ d }: { d: DecisionRecord }) {
  const [open, setOpen] = useState(false);
  const decisionsJson = useMemo(() => {
    try { return d.decisionJson ? JSON.parse(d.decisionJson) : []; } catch { return []; }
  }, [d]);
  const coins = [...new Set(d.candidateCoins ?? [])];
  const actions = decisionsJson as Record<string, unknown>[];

  return (
    <div className="decision-card">
      <button className="decision-head" onClick={() => setOpen((o) => !o)}>
        <span className="mono dim" style={{ fontSize: 11 }}>🤖 Cycle #{d.cycleNumber}</span>
        <span className="mono dim" style={{ fontSize: 11 }}>{fmtTime(d.timestamp)}</span>
        <span className="mono dim" style={{ fontSize: 11 }}>{actions.length} actions</span>
        {d.success ? <span className="mono" style={{ fontSize: 11, color: UP }}>✓ ok</span> : <span className="mono" style={{ fontSize: 11, color: DOWN }}>✗ {d.errorMessage ?? 'Failed'}</span>}
        <span className="mono dim" style={{ fontSize: 11, marginLeft: 'auto' }}>{open ? '▾' : '▸'}</span>
      </button>
      {/* 动作时间线（Execution Log 样式：每币一行 动作+符号+conf） */}
      {actions.length > 0 && (
        <div className="exec-timeline">
          {actions.map((a, i) => (
            <div key={i} className="exec-row">
              <span className="exec-t">{fmtTime(d.timestamp)}</span>
              <span className={`exec-act ${a.action === 'wait' || a.action === 'hold' ? 'wait' : 'live'}`}>{String(a.action)}</span>
              {a.symbol ? <span className="exec-sym">{String(a.symbol)}</span> : null}
              {typeof a.confidence === 'number' ? <span className="exec-conf">conf{Math.round((a.confidence as number) * 100)}</span> : null}
            </div>
          ))}
        </div>
      )}
      {coins.length > 0 && (
        <div className="chip-row" style={{ margin: '6px 0 2px' }}>
          {coins.map((c) => <span key={c} className="chip">{c}</span>)}
        </div>
      )}
      {/* AI 调用耗时 + 结果行 */}
      <div className="exec-foot">
        {d.aiRequestDurationMs ? <span className="mono dim" style={{ fontSize: 10.5 }}>AI call duration: {d.aiRequestDurationMs} ms</span> : null}
        <span className="mono" style={{ fontSize: 10.5, color: d.success ? UP : DOWN }}>
          {d.success ? `✓ ${(actions[0]?.symbol ?? 'ALL')} ${(actions[0]?.action ?? 'decision')} succeeded` : '✗ failed'}
        </span>
      </div>
      {open && (
        <div style={{ marginTop: 8, display: 'flex', flexDirection: 'column', gap: 6 }}>
          {d.systemPrompt && <PromptBlock label="SYSTEM PROMPT" text={d.systemPrompt} />}
          {d.inputPrompt && <PromptBlock label="USER PROMPT" text={d.inputPrompt} />}
          {d.cotTrace && <PromptBlock label="CHAIN OF THOUGHT" text={d.cotTrace} />}
          {!d.systemPrompt && !d.inputPrompt && !d.cotTrace && <div className="muted mono" style={{ fontSize: 11 }}>// 无详细轨迹</div>}
        </div>
      )}
    </div>
  );
}

function PromptBlock({ label, text }: { label: string; text: string }) {
  const [show, setShow] = useState(false);
  return (
    <div style={{ border: '1px solid var(--fxcore-panel-border)', borderRadius: 'var(--fxcore-r-sm)', overflow: 'hidden' }}>
      <button className="decision-head" onClick={() => setShow((s) => !s)} style={{ padding: '4px 10px', width: '100%' }}>
        <span className="mono dim" style={{ fontSize: 10.5, letterSpacing: '0.08em' }}>{label}</span>
        <span className="mono dim" style={{ fontSize: 10.5 }}>{show ? '▾' : '▸'}</span>
      </button>
      {show && (
        <pre style={{ margin: 0, padding: '8px 10px', maxHeight: 220, overflow: 'auto', fontSize: 10.5, lineHeight: 1.6, color: 'var(--fxcore-text-dim)', whiteSpace: 'pre-wrap', wordBreak: 'break-word', borderTop: '1px solid var(--fxcore-panel-border)' }}>{text}</pre>
      )}
    </div>
  );
}

function fmtSharpe(history: Position[]): string {
  const pnls = history.map((p) => (realizedOf(p) ?? 0)).filter((v) => Number.isFinite(v));
  if (pnls.length < 2) return '-';
  const mean = pnls.reduce((s, v) => s + v, 0) / pnls.length;
  const variance = pnls.reduce((s, v) => s + (v - mean) ** 2, 0) / (pnls.length - 1);
  const sd = Math.sqrt(variance);
  if (sd === 0) return '-';
  return (mean / sd * Math.sqrt(365)).toFixed(2);
}

/** tm 指标卡（tm-grid 风格：label 大写 + mono 大数值 + sub 副行） */
function TmCard({ label, value, sub, color }: { label: string; value: string; sub?: string; color?: string }) {
  return (
    <div className="tm-card">
      <div className="tm-card-label">{label}</div>
      <div className="tm-card-value" style={{ color: color ?? 'var(--fxcore-text)' }}>{value}</div>
      {sub && <div className="tm-card-sub">{sub}</div>}
    </div>
  );
}

/** Risk Radar 卡（语义标签：Ample/Calm/Tight 等状态词 + 数值副行） */
function RiskCard({ label, value, sub, color }: { label: string; value: string; sub?: string; color?: string }) {
  return (
    <div className="risk-card">
      <div className="tm-card-label">{label}</div>
      <div className="tm-card-value" style={{ fontSize: 15, color: color ?? 'var(--fxcore-text)' }}>{value}</div>
      {sub && <div className="tm-card-sub">{sub}</div>}
    </div>
  );
}

function PhStat({ label, value, color, hint }: { label: string; value: string; color?: string; hint?: string }) {
  return (
    <div style={{ padding: '6px 10px', background: 'var(--fxcore-panel-deep)', border: '1px solid var(--fxcore-panel-border)', borderRadius: 'var(--fxcore-r-sm)' }}>
      <div className="mono dim" style={{ fontSize: 9.5, letterSpacing: '0.1em' }}>{label}</div>
      <div className="mono" style={{ fontSize: 13.5, fontWeight: 600, color: color ?? 'var(--fxcore-text)', marginTop: 2 }}>{value}</div>
      {hint && <div className="mono dim" style={{ fontSize: 9.5, marginTop: 1 }}>{hint}</div>}
    </div>
  );
}

function CfgRow({ k, v }: { k: string; v: string }) {
  return (
    <div className="row" style={{ justifyContent: 'space-between', padding: '4px 0', fontSize: 12, borderBottom: '1px dashed var(--fxcore-panel-border)' }}>
      <span className="dim">{k}</span>
      <span className="mono">{v}</span>
    </div>
  );
}
