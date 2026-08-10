// 仪表盘段（SWR 5s 轮询，后台暂停）：统计卡 + 系统健康 + 运行中交易员 + 最近持仓。
import { useDashboard } from '../hooks/useDashboard';
import { PageHead, StatCard, Panel, Badge } from '../components/ui';

export default function DashboardSection() {
  const { data: dash, error, isLoading } = useDashboard();

  return (
    <section>
      <PageHead
        title="指挥中心"
        lead={<>实时聚合 <code>/dashboard</code> · SWR 5s 轮询（后台暂停）</>}
      />
      {error && <div className="alert error">错误：{String(error)}</div>}
      {isLoading && <div className="muted mono">· 加载中…</div>}
      {dash && (
        <>
          <div className="stat-grid">
            <StatCard label="总资产 / USDT" value={fmt(dash.account.totalBalance)} hint="全账户聚合" />
            <StatCard label="累计盈亏" value={fmtPnL(dash.account.totalPnl)} tone={tone(dash.account.totalPnl)} />
            <StatCard label="今日盈亏" value={fmtPnL(dash.account.dailyPnl)} tone={tone(dash.account.dailyPnl)} />
            <StatCard
              label="系统状态"
              value={dash.systemHealth.status === 'healthy' ? 'ONLINE' : 'DEGRADED'}
              tone={dash.systemHealth.status === 'healthy' ? 'up' : 'down'}
              hint={`AI ${dash.systemHealth.aiLatency}ms · 交易所 ${dash.systemHealth.exchangeLatency}ms`}
            />
          </div>

          <div className="stat-grid" style={{ gridTemplateColumns: 'repeat(auto-fit, minmax(280px, 1fr))' }}>
            <Panel title="运行中交易员">
              {dash.activeTraders.length === 0 ? (
                <div className="muted mono" style={{ padding: '10px 0' }}>暂无运行中交易员</div>
              ) : (
                <div className="table-wrap"><table className="data-table">
                  <thead>
                    <tr><th>名称</th><th>交易所</th><th>状态</th></tr>
                  </thead>
                  <tbody>
                    {dash.activeTraders.map((t) => (
                      <tr key={t.id}>
                        <td className="mono">{t.name}</td>
                        <td>{t.exchange}</td>
                        <td><Badge state={t.status} /></td>
                      </tr>
                    ))}
                  </tbody>
                </table></div>
              )}
            </Panel>

            <Panel title="最近持仓">
              {dash.recentPositions.length === 0 ? (
                <div className="muted mono" style={{ padding: '10px 0' }}>暂无持仓</div>
              ) : (
                <div className="table-wrap"><table className="data-table">
                  <thead>
                    <tr><th>币种</th><th>方向</th><th>数量</th><th>未实现盈亏</th></tr>
                  </thead>
                  <tbody>
                    {dash.recentPositions.map((p) => (
                      <tr key={p.id}>
                        <td className="mono">{p.symbol}</td>
                        <td style={{ color: p.side === 'long' ? 'var(--fxcore-up)' : 'var(--fxcore-down)' }}>
                          {p.side === 'long' ? '▲ 多' : '▼ 空'}
                        </td>
                        <td className="mono">{p.size}</td>
                        <td className="mono" style={{ color: (p.pnl ?? 0) >= 0 ? 'var(--fxcore-up)' : 'var(--fxcore-down)' }}>
                          {fmtPnL(p.pnl ?? 0)}
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table></div>
              )}
            </Panel>
          </div>
        </>
      )}
    </section>
  );
}

function fmt(v: number | undefined): string {
  if (typeof v !== 'number' || Number.isNaN(v)) return '-';
  return v.toLocaleString('en-US', { minimumFractionDigits: 2, maximumFractionDigits: 2 });
}
function fmtPnL(v: number | undefined): string {
  if (typeof v !== 'number' || Number.isNaN(v)) return '-';
  return `${v >= 0 ? '+' : ''}${fmt(v)}`;
}
function tone(v: number): 'up' | 'down' {
  return v >= 0 ? 'up' : 'down';
}
