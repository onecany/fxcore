// 交易所图标（public/exchange-icons/ 静态资源，位图 jpg/png 与矢量 svg 混用走 <img>）。
// 未知类型或加载失败 fallback：首字母方块（mono 金色，暗色底）。
import { useState } from 'react';

const ICON_FILES: Record<string, string> = {
  binance: 'binance.jpg',
  bybit: 'bybit.png',
  okx: 'okx.svg',
  bitget: 'bitget.svg',
  gate: 'gate.svg',
  kucoin: 'kucoin.svg',
  hyperliquid: 'hyperliquid.png',
  aster: 'aster.svg',
  indodax: 'indodax.svg',
  lighter: 'lighter.png',
};

export function ExchangeIcon({ type, size = 18 }: { type: string; size?: number }) {
  const file = ICON_FILES[type];
  const [failed, setFailed] = useState(false);
  if (!file || failed) {
    return (
      <span
        className="exchange-icon fallback"
        style={{ width: size, height: size, fontSize: Math.max(9, size * 0.5) }}
      >
        {(type[0] ?? '?').toUpperCase()}
      </span>
    );
  }
  return (
    <img
      className="exchange-icon"
      src={`/exchange-icons/${file}?v=1`}
      alt={type}
      width={size}
      height={size}
      loading="lazy"
      onError={() => setFailed(true)}
    />
  );
}
