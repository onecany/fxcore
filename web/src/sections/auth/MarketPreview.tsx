// 登录页实时行情预览卡：演示行情 + 2s 跳动 + 毛玻璃卡片。
import { useState } from 'react';
import KlineChart from '../../components/KlineChart';
import { DEMO_SYMBOLS, DEFAULT_SYMBOL, useDemoMarket } from './demo-market';
import type { TranslationKey } from '../../i18n/translations';

function fmtPrice(v: number): string {
  return v.toLocaleString('en-US', { minimumFractionDigits: 2, maximumFractionDigits: 2 });
}

function fmtTimeMs(ms: number): string {
  return new Date(ms).toLocaleTimeString('zh-CN', { hour12: false });
}

export function MarketPreview({ t }: { t: TranslationKey }) {
  const [symbol, setSymbol] = useState(DEFAULT_SYMBOL);
  const m = useDemoMarket(symbol);
  const up = m.changePct >= 0;
  const tone = up ? 'up' : 'down';

  return (
    <div className="glass-card mkt-card">
      <div className="mkt-head">
        <div className="mkt-title">
          <span className="mkt-live">
            <span className="led" />
            {t.landing.market.live}
          </span>
          {t.landing.market.title}
        </div>
        <span className="mkt-demo">{t.landing.market.demo}</span>
      </div>

      <div className="mkt-symbols">
        {DEMO_SYMBOLS.map((s) => (
          <button
            key={s}
            type="button"
            className={`mkt-chip ${s === symbol ? 'active' : ''}`}
            onClick={() => setSymbol(s)}
          >
            {s}
          </button>
        ))}
      </div>

      <div className="mkt-quote">
        <span key={m.tickAt} className={`mkt-price ${tone} tick-flash`}>
          {fmtPrice(m.last.close)}
        </span>
        <span className={`mkt-change ${tone}`}>
          {up ? '+' : ''}
          {m.changePct.toFixed(2)}%
        </span>
      </div>

      <div className="mkt-stats">
        <span>
          {t.landing.market.high} <b className="up">{fmtPrice(m.high)}</b>
        </span>
        <span>
          {t.landing.market.low} <b className="down">{fmtPrice(m.low)}</b>
        </span>
        <span>
          {t.landing.market.volume} <b>{fmtPrice(m.last.volume)}</b>
        </span>
      </div>

      <KlineChart data={m.bars} height={200} />

      <div className="mkt-foot">
        <span>
          {t.landing.market.updated} {fmtTimeMs(m.tickAt)}
        </span>
        <span className="mkt-symbol-mono">{symbol}·USDT</span>
      </div>
    </div>
  );
}
