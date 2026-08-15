// K 线行情面板（真实数据源链）：币种 chips + 周期下拉 + K 线图，10s 轮询。
import { useMemo, useState } from 'react';
import useSWR from 'swr';
import { listKlines } from '../../api/v1/modules/data';
import KlineChart from '../../components/KlineChart';
import { useT } from '../../stores/i18nStore';
import type { KlineDTO } from '../../api/v1/types/contract';

const INTERVALS = ['1m', '5m', '15m', '1h', '4h', '1d'] as const;
const FALLBACK_COINS = ['BTC-USDT', 'ETH-USDT', 'SOL-USDT', 'BNB-USDT'];

export function KlinePanel({ coins }: { coins?: string[] }) {
  const t = useT();
  const chips = useMemo(() => {
    const list = coins && coins.length > 0 ? coins : FALLBACK_COINS;
    return [...new Set([...list, ...FALLBACK_COINS])];
  }, [coins]);
  const [symbol, setSymbol] = useState(chips[0]);
  const [interval, setInterval] = useState('15m');

  const { data, isLoading, error } = useSWR<KlineDTO[]>(
    ['/klines', symbol, interval],
    () => listKlines({ symbol, interval, limit: 200 }),
    { refreshInterval: 10000 },
  );
  const klines = useMemo(() => data ?? [], [data]);

  return (
    <section className="glass-card kline-card">
      <div className="kline-head">
        <div className="kline-title">
          <span className="kline-live">
            <span className="led" />
            {t.kline.title}
          </span>
          <span className="mono dim" style={{ fontSize: 11 }}>{symbol} · {interval}</span>
        </div>
        <div className="kline-intervals">
          {INTERVALS.map((iv) => (
            <button
              key={iv}
              type="button"
              className={`kline-chip ${iv === interval ? 'active' : ''}`}
              onClick={() => setInterval(iv)}
            >
              {iv}
            </button>
          ))}
        </div>
      </div>
      <div className="kline-symbols">
        {chips.map((s) => (
          <button
            key={s}
            type="button"
            className={`kline-chip ${s === symbol ? 'active' : ''}`}
            onClick={() => setSymbol(s)}
          >
            {s}
          </button>
        ))}
      </div>
      {error && <div className="alert error">{t.kline.fetchFailed}：{String(error)}</div>}
      <div className="kline-panel">
        {isLoading && klines.length === 0 && <div className="kline-loading">{t.kline.loading}</div>}
        {klines.length > 0 ? (
          <KlineChart data={klines} height={220} />
        ) : !isLoading && (
          <div className="kline-loading">{t.kline.noData}</div>
        )}
      </div>
    </section>
  );
}
