// 交易终端数据 hook：SWR 缓存 + 轮询刷新（refreshInterval 5s），聚合 tm 指标/风控/历史统计。
// 分页（2026-08）：decisions 拉取分页（8/页，决策卡重）；orders/history 拉取上限 100 条
// （后端 size 上限）后表格内分页展示（10/页）；equity 全量拉取但 SVG 渲染抽稀到 400 点。
import { useEffect, useMemo, useState } from 'react';
import useSWR from 'swr';
import * as traderApi from '../../api/v1/modules/traders';
import * as dataApi from '../../api/v1/modules/data';
import type { TraderResponse, Position, EquitySnapshot } from '../../api/v1/types/contract';

/** 平仓盈亏双拼写兼容：/positions/history 裸 model 直出 realizedPnl（小写 l），DTO 风格 realizedPnL（大写 L） */
export function realizedOf(p: Position): number | undefined {
  return p.realizedPnL ?? p.realizedPnl;
}

/** 决策拉取分页大小（决策卡含可展开 Prompt，重量级，8/页） */
export const DEC_PAGE_SIZE = 8;
/** orders/history 拉取上限（后端 ListQuery size 上限 100） */
export const LIST_LIMIT = 100;
/** orders/history 表格展示分页大小 */
export const TABLE_PAGE_SIZE = 10;
/** equity SVG 渲染点数上限（抽稀） */
export const EQUITY_MAX_POINTS = 400;

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

/** 均匀降采样：超过上限时保留首尾 + 均匀中间点（SVG 曲线渲染不卡；指标计算仍用全量原始数据） */
function decimate(points: EquitySnapshot[], max: number): EquitySnapshot[] {
  if (points.length <= max) return points;
  const step = points.length / max;
  const out: EquitySnapshot[] = [];
  for (let i = 0; i < max; i++) {
    out.push(points[Math.min(points.length - 1, Math.floor(i * step))]);
  }
  out[out.length - 1] = points[points.length - 1]; // 末点必须是最新权益
  return out;
}

export function useTraderTerminal(traderId: string) {
  // 分页 state（decisions 为拉取分页；orders/history 为表格内分页）
  const [decisionsPage, setDecisionsPage] = useState(1);
  const [ordersPage, setOrdersPage] = useState(1);
  const [historyPage, setHistoryPage] = useState(1);
  // 切换交易员时重置分页（防串页缓存）
  useEffect(() => {
    setDecisionsPage(1);
    setOrdersPage(1);
    setHistoryPage(1);
  }, [traderId]);

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
          dataApi.listDecisions({ trader_id: traderId, page: decisionsPage, size: DEC_PAGE_SIZE }),
          traderApi.listPositions({}),
          dataApi.positionHistory({ trader_id: traderId, size: LIST_LIMIT }),
          dataApi.listOrders({ trader_id: traderId, size: LIST_LIMIT }),
        ]);
        return {
          equities: eq ?? [],
          decisions: (dc.items ?? []).filter((d) => d.traderId === traderId),
          decisionsTotalPages: dc.pagination?.totalPages ?? 1,
          positions: (ps ?? []).filter((p) => p.traderId === traderId && !p.closedAt),
          history: (hs.items ?? []).filter((p) => p.traderId === traderId),
          orders: (od.items ?? []).filter((o) => o.traderId === traderId),
        };
      }
    : null;

  const { data, error } = useSWR(
    traderId ? ['/terminal', traderId, decisionsPage, ordersPage, historyPage] : null,
    traderFetcher,
    { refreshInterval: traderId ? POLL : 0 },
  );

  // 全量原始数据（指标计算用，不抽稀）
  const rawEquities = useMemo(() => data?.equities ?? [], [data]);
  const decisions = useMemo(() => data?.decisions ?? [], [data]);
  const positions = useMemo(() => data?.positions ?? [], [data]);
  const history = useMemo(() => data?.history ?? [], [data]);
  const orders = useMemo(() => data?.orders ?? [], [data]);
  // SVG 渲染用抽稀数据
  const equities = useMemo(() => decimate(rawEquities, EQUITY_MAX_POINTS), [rawEquities]);

  const decisionsTotalPages = useMemo(() => data?.decisionsTotalPages ?? 1, [data]);
  const ordersTotalPages = useMemo(() => Math.max(1, Math.ceil(orders.length / TABLE_PAGE_SIZE)), [orders]);
  const historyTotalPages = useMemo(() => Math.max(1, Math.ceil(history.length / TABLE_PAGE_SIZE)), [history]);

  const trader = useMemo(() => traders.find((t) => t.id === traderId), [traders, traderId]);
  const latest = decisions[0];
  const equity = rawEquities[rawEquities.length - 1];
  const pnl = trader?.metrics?.totalPnl ?? 0;
  const unrealizedPnl = positions.reduce((s, p) => s + (p.unrealizedPnL ?? 0), 0);

  const maxDd = useMemo(() => {
    if (rawEquities.length < 2) return 0;
    let peak = rawEquities[0].equity;
    let dd = 0;
    for (const p of rawEquities) {
      if (p.equity > peak) peak = p.equity;
      if (peak > 0) dd = Math.max(dd, (peak - p.equity) / peak);
    }
    return dd;
  }, [rawEquities]);

  const equityChgPct = useMemo(() => {
    if (rawEquities.length < 2 || rawEquities[0].equity <= 0) return 0;
    return (rawEquities[rawEquities.length - 1].equity - rawEquities[0].equity) / rawEquities[0].equity;
  }, [rawEquities]);

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

  return {
    traders, mutateTraders, trader,
    equities, rawEquities, decisions, positions, history, orders,
    decisionsPage, setDecisionsPage, decisionsTotalPages,
    ordersPage, setOrdersPage, ordersTotalPages,
    historyPage, setHistoryPage, historyTotalPages,
    equity, latest, pnl, unrealizedPnl, maxDd, equityChgPct, stats, error,
  };
}
