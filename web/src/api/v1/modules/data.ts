// FXcore-web API v1 交易数据模块（交易员详情仪表盘用）。
import client from '../client';
import type { DecisionRecord, EquitySnapshot, Order, Fill, Position, PaginatedData, KlineDTO } from '../types/contract';

/** 决策记录（最新在前，trader_id 必填过滤） */
export function listDecisions(params: { trader_id: string; page?: number; size?: number }): Promise<PaginatedData<DecisionRecord>> {
  return client.get('/decisions', { params });
}

/** 权益快照（时间升序） */
export function equityHistory(traderId: string): Promise<EquitySnapshot[]> {
  return client.get('/equity-history', { params: { trader_id: traderId } });
}

/** 订单（最新在前，分页） */
export function listOrders(params: { trader_id?: string; symbol?: string; page?: number; size?: number }): Promise<PaginatedData<Order>> {
  return client.get('/orders', { params });
}

/** 某订单的成交 */
export function orderFills(orderId: string): Promise<Fill[]> {
  return client.get(`/orders/${orderId}/fills`);
}

/** 平仓历史（最新在前，分页） */
export function positionHistory(params: { trader_id?: string; symbol?: string; page?: number; size?: number }): Promise<PaginatedData<Position>> {
  return client.get('/positions/history', { params });
}

/** K 线行情（数据源链 hyperliquid→okx→coinank，真实数据；interval 如 15m/1h/4h） */
export function listKlines(params: { symbol: string; interval?: string; limit?: number }): Promise<KlineDTO[]> {
  return client.get('/klines', { params });
}
