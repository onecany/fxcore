// 多币种迷你行情卡（实时跳动演示）：价格 + 涨跌幅，绿涨红跌。
import { useDemoMarket } from './demo-market';

const TICKERS = [
  { symbol: 'BTC', glyph: '₿', name: 'Bitcoin' },
  { symbol: 'ETH', glyph: 'Ξ', name: 'Ethereum' },
  { symbol: 'SOL', glyph: '◎', name: 'Solana' },
  { symbol: 'BNB', glyph: '◈', name: 'BNB' },
];

function fmtPrice(v: number): string {
  return v.toLocaleString('en-US', { minimumFractionDigits: 2, maximumFractionDigits: 2 });
}

export function MiniTickers() {
  return (
    <div className="mini-tickers">
      {TICKERS.map((tk) => (
        <MiniTicker key={tk.symbol} tk={tk} />
      ))}
    </div>
  );
}

function MiniTicker({ tk }: { tk: (typeof TICKERS)[number] }) {
  const m = useDemoMarket(tk.symbol);
  const up = m.changePct >= 0;
  return (
    <div className="mini-ticker glass-card">
      <div className="mini-head">
        <span className="mini-glyph">{tk.glyph}</span>
        <span className="mini-name">
          {tk.symbol}
          <span className="mini-sub">{tk.name}</span>
        </span>
      </div>
      <div key={m.tickAt} className={`mini-price ${up ? 'up' : 'down'} tick-flash`}>
        ${fmtPrice(m.last.close)}
      </div>
      <div className={`mini-change ${up ? 'up' : 'down'}`}>
        {up ? '+' : ''}
        {m.changePct.toFixed(2)}%
      </div>
    </div>
  );
}
