// 交易终端数据 hook：SWR 缓存 + 轮询刷新（refreshInterval 5s），聚合 tm 指标/风控/历史统计。
import { useMemo } from 'react';
import useSWR from 'swr';
import * as traderApi from '../../api/v1/modules/traders';
import * as dataApi from '../../api/v1/modules/data';
import type { TraderResponse, Position } from '../../api/v1/types/contract';

/** 平仓盈亏双拼写兼容：/positions/history 裸 model 直出 realizedPnl（小写 l），DTO 风格 realizedPnL（大写 L） */
export function realizedOf(p: Position): number | undefined {
  return p.realizedPnL ?? p.realizedPnl;
}

export interface TerminalStats {
  total: number;
  winRate: number;
  totalPnl: number;
  fee: number;
  profitFactor: number;
  avgWin: number;
  avgLoss: number;
  netPnl: number;
  long: { count: number; winRate: number; pnl: number };
  short: { count: number; winRate: number; pnl: number };
  bySymbol: [string, { trades: number; wins: number; pnl: number }][];
}

const POLL = 5000;

export function useTraderTerminal(traderId: string) {
  // 交易员列表（不轮询，切换时刷新）
  const { data: traders = [], mutate: mutateTraders } = useSWR<TraderResponse[]>(
    ['/traders', 'terminal'],
    async () => {
      const d = await traderApi.listTraders({ size: 50 });
      return d.items;
    },
  );
  // 交易员详情详情数据（轮询）
  const traderFetcher = traderId
    ? async () => {
        const [eq, dc, ps, hs, od] = await Promise.all([
          dataApi.equityHistory(traderId),
          dataApi.listDecisions({ trader_id: traderId, size: 8 }),
          traderApi.listPositions({}),
          dataApi.positionHistory({ trader_id: traderId, limit: 50 }),
          dataApi.listOrders({ trader_id: traderId }),
        ]);
        return {
          equities: eq ?? [],
          decisions: (dc.items ?? []).filter((d) => d.traderId === traderId),
          positions: (ps ?? []).filter((p) => p.traderId === traderId && !p.closedAt),
          history: (hs.items ?? []).filter((p) => p.traderId === traderId),
          orders: (od.items ?? []).filter((o) => o.traderId === traderId),
        };
      }
    : null;

  const { data, error } = useSWR(traderId ? ['/terminal', traderId] : null, traderFetcher, {
    refreshInterval: traderId ? POLL : 0,
  });

  const equities = useMemo(() => data?.equities ?? [], [data]);
  const decisions = useMemo(() => data?.decisions ?? [], [data]);
  const positions = useMemo(() => data?.positions ?? [], [data]);
  const history = useMemo(() => data?.history ?? [], [data]);
  const orders = useMemo(() => data?.orders ?? [], [data]);

  const trader = useMemo(() => traders.find((t) => t.id === traderId), [traders, traderId]);
  const latest = decisions[0];
  const equity = equities[equities.length - 1];
  const pnl = trader?.metrics?.totalPnl ?? 0;
  const unrealizedPnl = positions.reduce((s, p) => s + (p.unrealizedPnL ?? 0), 0);

  const maxDd = useMemo(() => {
    if (equities.length < 2) return 0;
    let peak = equities[0].equity;
    let dd = 0;
    for (const p of equities) {
      if (p.equity > peak) peak = p.equity;
      if (peak > 0) dd = Math.max(dd, (peak - p.equity) / peak);
    }
    return dd;
  }, [equities]);

  const equityChgPct = useMemo(() => {
    if (equities.length < 2 || equities[0].equity <= 0) return 0;
    return (equities[equities.length - 1].equity - equities[0].equity) / equities[0].equity;
  }, [equities]);

  const stats = useMemo<TerminalStats>(() => {
    const closed = history;
    const total = closed.length;
    const wins = closed.filter((p) => (realizedOf(p) ?? 0) > 0);
    const losses = closed.filter((p) => (realizedOf(p) ?? 0) <= 0);
    const totalPnl = closed.reduce((s, p) => s + (realizedOf(p) ?? 0), 0);
    const fee = closed.reduce((s, p) => s + (p.fee ?? 0), 0);
    const winSum = wins.reduce((s, p) => s + (realizedOf(p) ?? 0), 0);
    const lossSum = Math.abs(losses.reduce((s, p) => s + (realizedOf(p) ?? 0), 0));
    const long = closed.filter((p) => p.side === 'long');
    const short = closed.filter((p) => p.side === 'short');
    const bySymbol = new Map<string, { trades: number; wins: number; pnl: number }>();
    for (const p of closed) {
      const e = bySymbol.get(p.symbol) ?? { trades: 0, wins: 0, pnl: 0 };
      e.trades += 1;
      if ((realizedOf(p) ?? 0) > 0) e.wins += 1;
      e.pnl += (realizedOf(p) ?? 0);
      bySymbol.set(p.symbol, e);
    }
    const winRate = total > 0 ? wins.length / total : 0;
    return {
      total,
      winRate,
      totalPnl,
      fee,
      profitFactor: lossSum > 0 ? winSum / lossSum : (winSum > 0 ? Infinity : 0),
      avgWin: wins.length > 0 ? winSum / wins.length : 0,
      avgLoss: losses.length > 0 ? -(lossSum / losses.length) : 0,
      netPnl: totalPnl - fee,
      long: { count: long.length, winRate: long.length > 0 ? long.filter((p) => (realizedOf(p) ?? 0) > 0).length / long.length : 0, pnl: long.reduce((s, p) => s + (realizedOf(p) ?? 0), 0) },
      short: { count: short.length, winRate: short.length > 0 ? short.filter((p) => (realizedOf(p) ?? 0) > 0).length / short.length : 0, pnl: short.reduce((s, p) => s + (realizedOf(p) ?? 0), 0) },
      bySymbol: [...bySymbol.entries()].sort((a, b) => b[1].pnl - a[1].pnl),
    };
  }, [history]);

  return { traders, mutateTraders, trader, equities, decisions, positions, history, orders, equity, latest, pnl, unrealizedPnl, maxDd, equityChgPct, stats, error };
}