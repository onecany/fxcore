// 交易所账户余额横条：SWR 拉取各所账户实时余额，失败项显示 error（降级）。
import useSWR from 'swr';
import { listBalances, type ExchangeBalance } from '../../api/v1/modules/exchanges';
import { ExchangeIcon } from '../../components/exchange-icon';
import { Panel } from '../../components/ui';

const fmtBal = (v: number | null): string => (v != null ? v.toLocaleString('en-US', { minimumFractionDigits: 2, maximumFractionDigits: 2 }) : '—');

export function ExchangeBalances() {
  const { data = [], error, isLoading } = useSWR<ExchangeBalance[]>(['/exchanges/balances'], () => listBalances(), {
    refreshInterval: 15000,
  });

  const total = data.reduce((s, x) => s + (x.balance ?? 0), 0);
  const okCount = data.filter((x) => x.balance != null).length;

  return (
    <Panel title={`交易所余额 · ${okCount}/${data.length} 已查询`} className="mb">
      <div className="exb-bar">
        <div className="exb-total mono">
          <span className="dim">合计（USDT）</span>
          <span className="exb-total-val">{isLoading ? '…' : data.length > 0 ? fmtBal(total) : '—'}</span>
        </div>
        {error && <div className="mono dim" style={{ fontSize: 11 }}>查询异常：{String(error)}</div>}
        {!error && data.length === 0 && !isLoading && <div className="muted mono" style={{ fontSize: 12 }}>// 无交易所账户</div>}
        {data.map((b) => (
          <div key={`${b.exchangeType}-${b.accountName}`} className="exb-item" title={b.error}>
            <ExchangeIcon type={b.exchangeType} size={16} />
            <span className="exb-name">{b.accountName}</span>
            <span className={`exb-bal mono ${b.balance != null ? '' : 'err'}`}>
              {b.balance != null ? `${fmtBal(b.balance)} ${b.currency}` : '查询失败'}
            </span>
          </div>
        ))}
      </div>
    </Panel>
  );
}