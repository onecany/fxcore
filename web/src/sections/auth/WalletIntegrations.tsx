// 登录页钱包集成演示：10 家交易所图标墙 + 模拟账户摘要（纯前端演示，标注 demo）。
import { ExchangeIcon } from '../../components/exchange-icon';
import type { TranslationKey } from '../../i18n/translations';

const EXCHANGES = [
  'binance', 'bybit', 'okx', 'bitget', 'gate', 'kucoin', 'hyperliquid', 'aster', 'indodax', 'lighter',
];

// 模拟资产摘要（演示数据，非真实账户）
const BALANCES = [
  { asset: 'BTC', amount: '0.1284', usd: 8654 },
  { asset: 'ETH', amount: '2.3100', usd: 8134 },
  { asset: 'USDT', amount: '12,480.32', usd: 12480 },
];

export function WalletIntegrations({ t }: { t: TranslationKey }) {
  const total = BALANCES.reduce((sum, b) => sum + b.usd, 0);
  return (
    <section className="landing-block">
      <h3 className="block-title">{t.landing.wallet.title}</h3>
      <p className="block-lead">{t.landing.wallet.lead}</p>
      <div className="glass-card wallet-card">
        <div className="wallet-left">
          <div className="wallet-label">{t.landing.wallet.balances}</div>
          <div className="wallet-total">≈ ${total.toLocaleString('en-US')}</div>
          <div className="exch-wall">
            {EXCHANGES.map((x) => (
              <span key={x} className="exch-cell" title={x}>
                <ExchangeIcon type={x} size={22} />
                <span className="exch-name">{x}</span>
              </span>
            ))}
          </div>
        </div>
        <div className="wallet-right">
          <div className="wallet-status">
            <span className="led" />
            {t.landing.wallet.connected}
            <span className="mkt-demo">{t.landing.wallet.demo}</span>
          </div>
          <div className="wallet-rows">
            {BALANCES.map((b) => (
              <div key={b.asset} className="wallet-row">
                <span className="wallet-asset">{b.asset}</span>
                <span className="wallet-amount">{b.amount}</span>
                <span className="wallet-usd">${b.usd.toLocaleString('en-US')}</span>
              </div>
            ))}
          </div>
        </div>
      </div>
    </section>
  );
}
