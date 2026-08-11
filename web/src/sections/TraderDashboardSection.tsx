// 交易员详情仪表盘（指挥中心信息架构）：
// 顶栏：交易员下拉 + 状态 + tm 摘要行 + tm 指标网格 + Risk Radar
// → 非对称主次两列：主列（Current Positions + Execution Log）/ 辅助列（权益曲线 + 配置摘要）
// → Position History（全宽统计网格 + LONG/SHORT 分拆 + Symbol Performance + 明细表）
// 数据层：SWR 缓存 + 5s 轮询（useTraderTerminal），支持 URL ?trader=<id> 直达。
import { useCallback, useEffect, useMemo, useState } from 'react';
import { PageHead, Panel, Badge } from '../components/ui';
import { fmtUsd, fmtPct, fmtDur } from '../utils/format';
import { useTraderTerminal, realizedOf } from './dashboard/useTraderTerminal';
import { UP, DOWN, TmCard, RiskCard, PhStat, CfgRow, EquityCurve } from './dashboard/primitives';
import { DecisionCard, fmtClock, pnlColor, fmtSharpe } from './dashboard/exec-log';

export default function TraderDashboardSection() {
  const [traderId, setTraderId] = useTraderIdFromURL();
  const {
    traders, trader, equities, decisions, positions, history, orders,
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

  return (
    <section>
      <PageHead
        title="交易终端"
        lead={<>交易员详情 · URL <code>?trader=</code> 直达 · 实时刷新</>}
      />
      {error && <div className="alert error">{error}</div>}

      {/* 顶栏：orchestration select + 名称 ID + 状态 */}
      <div className="row wrap" style={{ gap: 12, marginBottom: 14 }}>
        <label className="dim" style={{ fontSize: 12 }}>orchestration</label>
        <select
          value={traderId}
          onChange={(e) => onSelectTrader(e.target.value)}
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
        <TmCard label="TOTAL P&L · INCL. UNREALIZED" value={fmtUsd(pnl + unrealizedPnl, true)} sub={fmtPct(equity?.equity ? (pnl + unrealizedPnl) / equity.equity : 0)} color={pnlColor(pnl + unrealizedPnl)} />
        <TmCard label="TOTAL P&L · CLOSED" value={fmtUsd(stats.totalPnl, true)} color={pnlColor(stats.totalPnl)} />
        <TmCard label="PROFIT FACTOR" value={Number.isFinite(stats.profitFactor) ? stats.profitFactor.toFixed(2) : '-'} sub="总盈/总亏" />
        <TmCard label="MAX DRAWDOWN" value={fmtPct(maxDd)} color={maxDd > 0.1 ? DOWN : undefined} sub="自峰回撤" />
      </div>

      {/* tm 第二行指标：trades / win / loss / net / sharpe / avg win-loss */}
      <div className="tm-mono tm-line2" style={{ marginBottom: 14 }}>
        <span className="tm-dim">trades</span><span className="tm-v">{stats.total}</span>
        <span className="tm-dim">win</span><span className="tm-v ok">{wins.length} ({fmtPct(stats.winRate)})</span>
        <span className="tm-dim">loss</span><span className="tm-v down">{losses.length}</span>
        <span className="tm-dim">net</span><span className="tm-v" style={{ color: pnlColor(stats.netPnl) }}>{fmtUsd(stats.netPnl, true)}</span>
        <span className="tm-dim">fees</span><span className="tm-v">{fmtUsd(stats.fee)}</span>
        <span className="tm-dim">sharpe/trade</span><span className="tm-v">{fmtSharpe(history.map((p) => realizedOf(p) ?? 0))}</span>
        <span className="tm-dim">avg win/loss</span><span className="tm-v">{fmtUsd(stats.avgWin, true)} / {fmtUsd(stats.avgLoss, true)}</span>
      </div>

      {/* Risk Radar：风险雷达（从 equity/positions/history 聚合） */}
      <div className="tm-grid-7" style={{ marginBottom: 14 }}>
        <RiskCard label="NET EXPOSURE" value="Flat" sub={`long ${fmtUsd(0)} / short ${fmtUsd(0)}`} />
        <RiskCard label="LEVERAGE" value="—" sub={`${fmtPct(equity?.marginUsedPct)} avg / — peak`} />
        <RiskCard label="MARGIN USED" value={equity?.marginUsedPct != null && equity.marginUsedPct < 0.3 ? 'Ample' : equity?.marginUsedPct != null && equity.marginUsedPct < 0.7 ? 'Moderate' : 'Tight'} sub={`${fmtPct(equity?.marginUsedPct)} of equity`} />
        <RiskCard label="MAX DRAWDOWN" value={maxDd < 0.05 ? 'Calm' : maxDd < 0.15 ? 'Watch' : 'Alert'} sub={`${fmtPct(maxDd)} peak drawdown`} />
        <RiskCard label="POSITIONS" value={`${positions.length}`} sub={`${positions.length} held / cap`} />
        <RiskCard label="UNREALIZED PNL" value={fmtUsd(unrealizedPnl, true)} color={pnlColor(unrealizedPnl)} />
        <RiskCard label="AVAILABLE" value={fmtUsd(equity?.balance)} sub={`${fmtPct(equity?.marginUsedPct)} 占用`} />
      </div>

      {/* 非对称主次：主列（持仓+执行日志）宽，辅助列（权益+配置）窄 */}
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

        {/* 辅助列：权益曲线 + 配置摘要 + 订单 */}
        <div className="td-col-side">
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
        </div>
      </div>

      {/* Position History 统计（全宽） */}
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
