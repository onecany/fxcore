// 交易员详情仪表盘（激进版布局）：
// ① 交易员选择条（毛玻璃横条）→ ② 交易所余额 → ③ KPI 6 卡 → ④ 终端状态行 → ⑤ K 线行情全宽
// → ⑥ 非对称两列（主列持仓+执行日志 / 辅助列权益+风险+配置）→ ⑦ Position History 全宽
// 数据层：SWR 缓存 + 5s 轮询（useTraderTerminal），K 线 10s 轮询，支持 URL ?trader=<id> 直达。
import { useCallback, useEffect, useMemo, useState } from 'react';
import useSWR from 'swr';
import { PageHead, Panel } from '../components/ui';
import { fmtUsd, fmtPct, fmtDur } from '../utils/format';
import { errMsg } from '../utils/errorCodes';
import { useTraderTerminal, realizedOf, TABLE_PAGE_SIZE } from './dashboard/useTraderTerminal';
import { UP, DOWN, TmCard, PhStat, CfgRow, EquityCurve, Pager } from './dashboard/primitives';
import { DecisionCard, fmtClock, pnlColor, fmtSharpe } from './dashboard/exec-log';
import { ExchangeBalances } from './dashboard/exchange-balances';
import { TraderSelect } from './dashboard/TraderSelect';
import { KlinePanel } from './dashboard/KlinePanel';
import * as modelApi from '../api/v1/modules/models';
import * as strategyApi from '../api/v1/modules/strategies';
import type { AIModel, StrategyItem } from '../api/v1/types/contract';

export default function TraderDashboardSection() {
  const [traderId, setTraderId] = useTraderIdFromURL();
  const {
    traders, trader, equities, decisions, positions, history, orders,
    decisionsPage, setDecisionsPage, decisionsTotalPages,
    ordersPage, setOrdersPage, ordersTotalPages,
    historyPage, setHistoryPage, historyTotalPages,
    equity, latest, pnl, unrealizedPnl, maxDd, equityChgPct, stats, error,
  } = useTraderTerminal(traderId);

  // 自动选中：URL ?trader= 优先，否则首个 running，否则第一个（原 loadTraders 行为）
  useEffect(() => {
    if (traderId || traders.length === 0) return;
    const q = new URLSearchParams(window.location.search).get('trader');
    const preferred = q ? traders.find((t) => t.id === q) : traders.find((t) => t.status === 'running');
    setTraderId(preferred?.id ?? traders[0].id);
  }, [traderId, traders, setTraderId]);

  const onSelectTrader = useCallback((id: string) => {
    setTraderId(id);
  }, [setTraderId]);

  const wins = useMemo(() => history.filter((p) => (realizedOf(p) ?? 0) > 0), [history]);
  const losses = useMemo(() => history.filter((p) => (realizedOf(p) ?? 0) <= 0), [history]);

  // 模型/策略名称映射：配置摘要显示存储名称而非 id（2026-08 用户要求）
  const { data: models } = useSWR<AIModel[]>(['/models', 'terminal-name'], async () => modelApi.listModels());
  const { data: strategies } = useSWR<StrategyItem[]>(['/strategies', 'terminal-name'], async () => {
    const d = await strategyApi.listStrategies();
    return d.items;
  });
  const modelNameOf = useCallback((id?: string) => {
    if (!id) return '-';
    return models?.find((m) => m.id === id)?.name ?? id;
  }, [models]);
  const strategyNameOf = useCallback((id?: string) => {
    if (!id) return '-';
    return strategies?.find((s) => s.id === id)?.name ?? id;
  }, [strategies]);
  // 周期展示：后端 interval 为秒，UI 用分钟（用户偏好）；<60s 显示秒
  const cycleText = useMemo(() => {
    const sec = trader?.schedule?.interval;
    if (!sec || sec <= 0) return '-';
    if (sec < 60) return `${sec}s`;
    const min = sec / 60;
    return Number.isInteger(min) ? `${min}m` : `${min.toFixed(1)}m`;
  }, [trader]);

  const statusLabel = trader?.status === 'running' ? 'ONLINE' : (trader?.status ?? 'OFFLINE').toUpperCase();
  const statusTone = trader?.status === 'running' ? 'ok' : trader?.status === 'paused' ? 'warn' : '';

  // 风险状态（真实数据聚合，替代硬编码演示值）
  const marginState = equity?.marginUsedPct == null ? '—'
    : equity.marginUsedPct < 0.3 ? 'Ample' : equity.marginUsedPct < 0.7 ? 'Moderate' : 'Tight';
  const ddState = maxDd < 0.05 ? 'Calm' : maxDd < 0.15 ? 'Watch' : 'Alert';

  return (
    <section>
      <PageHead
        title="交易终端"
        lead={<>交易员详情 · URL <code>?trader=</code> 直达 · 实时刷新</>}
      />
      {error && <div className="alert error">{errMsg(error)}</div>}

      {/* ① 交易员选择条：选择器 + 名称/交易所 + 状态 + cycle */}
      <div className="glass-card dash-topbar">
        <TraderSelect traders={traders} traderId={traderId} onSelect={onSelectTrader} />
        <div className="dash-tb-meta">
          <span className="dash-tb-name">{trader?.name ?? '-'}</span>
          <span className="dash-tb-ex">{trader?.exchange ?? '-'}</span>
          <span className={`dash-tb-status ${statusTone}`}>{statusLabel}</span>
          <span className="dash-tb-cycle">
            cycle <b>{cycleText}</b>
            {latest?.cycleNumber != null && <em>· 第 {latest.cycleNumber} 轮</em>}
          </span>
          <span className="dash-tb-id mono">{trader ? `ID ${trader.id.slice(0, 8)}…` : ''}</span>
        </div>
      </div>

      {/* ② 交易所余额横条（全局账户级，不依赖具体交易员） */}
      <ExchangeBalances />

      {/* ③ KPI 主指标行（6 卡，毛玻璃） */}
      <div className="dash-kpi">
        <TmCard label="EQUITY" value={fmtUsd(equity?.equity)} sub={`${fmtPct(equityChgPct)} ${equityChgPct >= 0 ? '▲' : '▼'}`} color={equityChgPct >= 0 ? UP : DOWN} />
        <TmCard label="TOTAL P&L · INCL. UNREALIZED" value={fmtUsd(pnl + unrealizedPnl, true)} sub={fmtPct(equity?.equity ? (pnl + unrealizedPnl) / equity.equity : 0)} color={pnlColor(pnl + unrealizedPnl)} />
        <TmCard label="TOTAL P&L · CLOSED" value={fmtUsd(stats.totalPnl, true)} color={pnlColor(stats.totalPnl)} />
        <TmCard label="WIN RATE" value={fmtPct(stats.winRate)} sub={`Win ${wins.length} / Loss ${losses.length}`} color={stats.winRate >= 0.5 ? UP : DOWN} />
        <TmCard label="PROFIT FACTOR" value={Number.isFinite(stats.profitFactor) ? stats.profitFactor.toFixed(2) : '-'} sub="总盈/总亏" />
        <TmCard label="MAX DRAWDOWN" value={fmtPct(maxDd)} color={maxDd > 0.1 ? DOWN : undefined} sub="自峰回撤" />
      </div>

      {/* ④ 终端状态行：模型/策略/持仓/权益/盈亏/交易统计（mono 终端风） */}
      <div className="glass-card dash-status tm-mono">
        <span className="tm-dim">model</span><span className="tm-v">{modelNameOf(trader?.modelConfig?.modelId)}</span>
        <span className="tm-dim">strategy</span><span className="tm-v">{strategyNameOf(trader?.strategyId)}</span>
        <span className="tm-dim">positions</span><span className="tm-v">{positions.length}</span>
        <span className="tm-dim">eq</span><span className="tm-v">{fmtUsd(equity?.equity)}</span>
        <span className="tm-dim">pnl</span><span className="tm-v" style={{ color: pnlColor(pnl) }}>{fmtUsd(pnl, true)}</span>
        <span className="tm-dim">trades</span><span className="tm-v">{stats.total}</span>
        <span className="tm-dim">win</span><span className="tm-v ok">{wins.length} ({fmtPct(stats.winRate)})</span>
        <span className="tm-dim">loss</span><span className="tm-v down">{losses.length}</span>
        <span className="tm-dim">net</span><span className="tm-v" style={{ color: pnlColor(stats.netPnl) }}>{fmtUsd(stats.netPnl, true)}</span>
        <span className="tm-dim">fees</span><span className="tm-v">{fmtUsd(stats.fee)}</span>
        <span className="tm-dim">sharpe/trade</span><span className="tm-v">{fmtSharpe(history.map((p) => realizedOf(p) ?? 0))}</span>
        <span className="tm-dim">avg win/loss</span><span className="tm-v">{fmtUsd(stats.avgWin, true)} / {fmtUsd(stats.avgLoss, true)}</span>
      </div>

      {/* ⑤ K 线行情面板（全宽，真实数据源链） */}
      <KlinePanel />

      {/* ⑥ 非对称主次：主列（持仓+执行日志）宽，辅助列（权益+风险+配置）窄 */}
      <div className="td-grid td-grid-main">
        {/* 主列：Current Positions + Execution Log */}
        <div className="td-col-main">
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
            ) : <div className="muted mono empty-hint">📊 No Positions · 无活动持仓</div>}
          </Panel>
          <Panel title={`Execution Log (${decisions.length} cyc · ${decisionsPage}/${decisionsTotalPages})`}>
            {decisions.length > 0 ? (
              <div className="dash-stack">
                {decisions.map((d) => <DecisionCard key={d.id} d={d} />)}
              </div>
            ) : <div className="muted mono empty-hint">// 暂无决策记录</div>}
            <Pager page={decisionsPage} totalPages={decisionsTotalPages} onPage={setDecisionsPage} />
          </Panel>
        </div>

        {/* 辅助列：权益曲线 + 风险状态 + 配置摘要 */}
        <div className="td-col-side">
          <Panel title="权益曲线">
            <EquityCurve points={equities} height={160} />
            <div className="dash-eq-meta">
              <span>Initial {fmtUsd(equities[0]?.equity)}</span>
              <span>· Current {fmtUsd(equity?.equity)}</span>
              <span>· Cycles {equities.length}</span>
              <span>· Margin {fmtPct(equity?.marginUsedPct)}</span>
            </div>
          </Panel>
          <Panel title="风险状态">
            <div className="dash-risk">
              <div className="dash-risk-item">
                <span className="dash-risk-label">MARGIN USED</span>
                <span className="dash-risk-val">{marginState}</span>
                <span className="dash-risk-sub">{fmtPct(equity?.marginUsedPct)} of equity</span>
              </div>
              <div className="dash-risk-item">
                <span className="dash-risk-label">MAX DRAWDOWN</span>
                <span className="dash-risk-val" style={{ color: ddState === 'Alert' ? DOWN : ddState === 'Watch' ? 'var(--fxcore-warn)' : undefined }}>{ddState}</span>
                <span className="dash-risk-sub">{fmtPct(maxDd)} peak drawdown</span>
              </div>
              <div className="dash-risk-item">
                <span className="dash-risk-label">POSITIONS</span>
                <span className="dash-risk-val">{positions.length}</span>
                <span className="dash-risk-sub">{positions.length} held</span>
              </div>
              <div className="dash-risk-item">
                <span className="dash-risk-label">UNREALIZED PNL</span>
                <span className="dash-risk-val" style={{ color: pnlColor(unrealizedPnl) }}>{fmtUsd(unrealizedPnl, true)}</span>
                <span className="dash-risk-sub">open positions</span>
              </div>
              <div className="dash-risk-item">
                <span className="dash-risk-label">AVAILABLE</span>
                <span className="dash-risk-val">{equity?.balance != null ? fmtUsd(equity.balance) : '0.00'}</span>
                <span className="dash-risk-sub">{equity?.balance != null ? `${fmtPct(equity.marginUsedPct)} 占用` : '暂无权益快照（未运行）'}</span>
              </div>
            </div>
          </Panel>
          <Panel title="配置摘要">
            <CfgRow k="交易所" v={trader?.exchange ?? '-'} />
            <CfgRow k="模型" v={`${trader?.modelConfig?.provider ?? '-'} / ${modelNameOf(trader?.modelConfig?.modelId)}`} />
            <CfgRow k="策略" v={strategyNameOf(trader?.strategyId)} />
            <CfgRow k="周期" v={cycleText === '-' ? '-' : `${cycleText}（${trader?.schedule?.interval} 秒）`} />
            <CfgRow k="单笔上限" v={`${fmtUsd(trader?.riskConfig?.maxPositionSize)} USDT`} />
            <CfgRow k="止损" v={fmtPct(trader?.riskConfig?.stopLoss)} />
            <CfgRow k="止盈" v={fmtPct(trader?.riskConfig?.takeProfit)} />
            <CfgRow k="日损上限" v={fmtPct(trader?.riskConfig?.maxDailyLoss)} />
          </Panel>
          <Panel title={`订单 (${orders.length})`}>
            {orders.length > 0 ? (
              <div className="table-wrap"><table className="data-table">
                <thead><tr><th>币种</th><th>类型</th><th>状态</th><th>数量</th><th>价格</th></tr></thead>
                <tbody>
                  {orders.slice((ordersPage - 1) * TABLE_PAGE_SIZE, ordersPage * TABLE_PAGE_SIZE).map((o) => (
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
            ) : <div className="muted mono empty-hint">// NO ORDERS</div>}
            <Pager page={ordersPage} totalPages={ordersTotalPages} onPage={setOrdersPage} />
          </Panel>
        </div>
      </div>

      {/* ⑦ Position History 统计（全宽） */}
      <div className="td-grid td-grid-1">
        <Panel title={`Position History (${stats.total})`}>
          {stats.total > 0 ? (
            <>
              <div className="ph-stats">
                <PhStat label="WIN RATE" value={fmtPct(stats.winRate)} hint={`Win ${wins.length} / Loss ${losses.length}`} />
                <PhStat label="NET P&L" value={fmtUsd(stats.netPnl, true)} color={pnlColor(stats.netPnl)} hint={`Fee ${fmtUsd(stats.fee)}`} />
                <PhStat label="PROFIT FACTOR" value={Number.isFinite(stats.profitFactor) ? stats.profitFactor.toFixed(2) : '-'} hint="总盈/总亏" />
                <PhStat label="AVG WIN" value={fmtUsd(stats.avgWin, true)} color={UP} />
                <PhStat label="AVG LOSS" value={fmtUsd(stats.avgLoss, true)} color={DOWN} />
                <PhStat label="SHARPE" value={fmtSharpe(history.map((p) => realizedOf(p) ?? 0))} hint="风险调整收益" />
              </div>
              <div className="dash-chips">
                <span className="chip">{`LONG ${stats.long.count} · WR ${fmtPct(stats.long.winRate)} · ${fmtUsd(stats.long.pnl, true)}`}</span>
                <span className="chip">{`SHORT ${stats.short.count} · WR ${fmtPct(stats.short.winRate)} · ${fmtUsd(stats.short.pnl, true)}`}</span>
              </div>
              {stats.bySymbol.length > 0 && (
                <div className="dash-block">
                  <div className="mono dim dash-subhead">SYMBOL PERFORMANCE</div>
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
              <div className="dash-block">
                <div className="mono dim dash-subhead">CLOSED TRADES</div>
                <div className="table-wrap"><table className="data-table">
                  <thead><tr><th>币种</th><th>方向</th><th>盈亏</th><th>Fee</th><th>时长</th><th>平仓</th></tr></thead>
                  <tbody>
                    {history.slice((historyPage - 1) * TABLE_PAGE_SIZE, historyPage * TABLE_PAGE_SIZE).map((p) => (
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
                <Pager page={historyPage} totalPages={historyTotalPages} onPage={setHistoryPage} />
              </div>
            </>
          ) : <div className="muted mono empty-hint">// NO CLOSED POSITIONS</div>}
        </Panel>
      </div>
    </section>
  );
}

/** URL ?trader=<id> 直达：初始从 query 取（惰性），切换时写回（replaceState 不触发重载） */
function useTraderIdFromURL(): [string, (id: string) => void] {
  const [traderId, setTraderId] = useState<string>(() => new URLSearchParams(window.location.search).get('trader') ?? '');
  const update = useCallback((next: string) => {
    setTraderId(next);
    window.history.replaceState(null, '', `?trader=${next}`);
  }, []);
  return [traderId, update];
}
