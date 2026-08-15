// 登录页实时行情预览：前端内置演示行情（确定性随机游走 + 定时跳动）。
// 未登录态无法访问鉴权行情端点，预览卡不依赖后端；数据形状对齐 KlineDTO，KlineChart 直接复用。
// 历史序列用种子 PRNG 保证同币种每次渲染稳定；实时跳动用独立随机，每 2s 推一根新 K 线。
import { useEffect, useMemo, useState } from 'react';
import type { KlineDTO } from '../../api/v1/types/contract';

/** 币种配置：起始价 / 波动率 / 轻微漂移 */
const MARKET_CFG: Record<string, { base: number; vol: number; drift: number }> = {
  BTC: { base: 67420, vol: 0.0035, drift: 0.00015 },
  ETH: { base: 3521, vol: 0.0042, drift: 0.0002 },
  SOL: { base: 184.5, vol: 0.006, drift: 0.0003 },
  BNB: { base: 598, vol: 0.0045, drift: 0.0002 },
};

export const DEMO_SYMBOLS = Object.keys(MARKET_CFG);

export const DEFAULT_SYMBOL = 'BTC';

function cfgOf(symbol: string) {
  return MARKET_CFG[symbol] ?? MARKET_CFG[DEFAULT_SYMBOL];
}

/** 确定性 PRNG（mulberry32）：同种子产出同序列 */
function mulberry32(seed: number) {
  let a = seed >>> 0;
  return () => {
    a |= 0;
    a = (a + 0x6d2b79f5) | 0;
    let t = Math.imul(a ^ (a >>> 15), 1 | a);
    t = (t + Math.imul(t ^ (t >>> 7), 61 | t)) ^ t;
    return ((t ^ (t >>> 14)) >>> 0) / 4294967296;
  };
}

function hashSymbol(symbol: string): number {
  let h = 0;
  for (let i = 0; i < symbol.length; i++) h = (h * 31 + symbol.charCodeAt(i)) | 0;
  return h >>> 0;
}

/** 生成一段分钟级 OHLCV 历史（升序） */
export function genHistory(symbol: string, bars = 80): KlineDTO[] {
  const cfg = cfgOf(symbol);
  const rand = mulberry32(hashSymbol(symbol));
  const now = Math.floor(Date.now() / 60000) * 60;
  const out: KlineDTO[] = [];
  let price = cfg.base * (0.97 + rand() * 0.06);
  for (let i = 0; i < bars; i++) {
    const open = price;
    const drift = cfg.drift * (rand() - 0.45);
    const shock = (rand() - 0.5) * 2 * cfg.vol;
    const close = Math.max(open * (1 + drift + shock), 1e-6);
    const high = Math.max(open, close) * (1 + rand() * cfg.vol * 0.6);
    const low = Math.min(open, close) * (1 - rand() * cfg.vol * 0.6);
    out.push({
      timestamp: now - (bars - i) * 60,
      open,
      high,
      low,
      close,
      volume: 40 + rand() * 260,
    });
    price = close;
  }
  return out;
}

/** 基于最后一根生成新一根（实时跳动） */
export function nextBar(prev: KlineDTO, symbol: string): KlineDTO {
  const cfg = cfgOf(symbol);
  const open = prev.close;
  const drift = cfg.drift * (Math.random() - 0.45);
  const shock = (Math.random() - 0.5) * 2 * cfg.vol;
  const close = Math.max(open * (1 + drift + shock), 1e-6);
  const high = Math.max(open, close) * (1 + Math.random() * cfg.vol * 0.5);
  const low = Math.min(open, close) * (1 - Math.random() * cfg.vol * 0.5);
  return {
    timestamp: prev.timestamp + 60,
    open,
    high,
    low,
    close,
    volume: 40 + Math.random() * 260,
  };
}

export interface DemoMarket {
  bars: KlineDTO[];
  last: KlineDTO;
  /** 窗口内涨跌幅 % */
  changePct: number;
  /** 窗口最高 / 最低 */
  high: number;
  low: number;
  /** 最后跳动时间（组件用它触发价格闪烁动画） */
  tickAt: number;
}

/** 演示行情 hook：2s 一跳，维持固定窗口 */
export function useDemoMarket(symbol: string): DemoMarket {
  const [bars, setBars] = useState<KlineDTO[]>(() => genHistory(symbol));
  const [tickAt, setTickAt] = useState(() => Date.now());

  useEffect(() => {
    setBars(genHistory(symbol));
    setTickAt(Date.now());
    const id = window.setInterval(() => {
      setBars((prev) => {
        const nb = nextBar(prev[prev.length - 1], symbol);
        return [...prev.slice(-79), nb];
      });
      setTickAt(Date.now());
    }, 2000);
    return () => window.clearInterval(id);
  }, [symbol]);

  return useMemo(() => {
    const last = bars[bars.length - 1];
    const first = bars[0];
    const changePct = first ? ((last.close - first.close) / first.close) * 100 : 0;
    let high = last.high;
    let low = last.low;
    for (const b of bars) {
      if (b.high > high) high = b.high;
      if (b.low < low) low = b.low;
    }
    return { bars, last, changePct, high, low, tickAt };
  }, [bars, tickAt]);
}
