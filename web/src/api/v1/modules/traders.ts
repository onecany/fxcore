// FXcore-web API v1 交易员模块（CRUD + 状态机控制 + 持仓）。
import client from '../client';
import type {
  CreateTraderRequest,
  PaginatedData,
  Position,
  TraderResponse,
  TraderStatus,
} from '../types/contract';

export interface TraderListParams {
  page?: number;
  size?: number;
  status?: TraderStatus;
  fields?: string;
}

/** 交易员列表（分页 + 状态过滤 + 字段过滤） */
export function listTraders(params: TraderListParams = {}): Promise<PaginatedData<TraderResponse>> {
  return client.get('/traders', { params });
}

/** 单个交易员 */
export function getTrader(id: string): Promise<TraderResponse> {
  return client.get(`/traders/${id}`);
}

/** 创建交易员 */
export function createTrader(req: CreateTraderRequest): Promise<TraderResponse> {
  return client.post('/traders', req);
}

/** 部分更新（Partial<CreateTraderRequest>） */
export function updateTrader(id: string, req: Partial<CreateTraderRequest>): Promise<TraderResponse> {
  return client.patch(`/traders/${id}`, req);
}

/** 启动（幂等：运行中返回 1204） */
export function startTrader(id: string): Promise<null> {
  return client.post(`/traders/${id}/start`);
}

/** 暂停 */
export function pauseTrader(id: string): Promise<null> {
  return client.post(`/traders/${id}/pause`);
}

/** 恢复 */
export function resumeTrader(id: string): Promise<null> {
  return client.post(`/traders/${id}/resume`);
}

/** 停止 */
export function stopTrader(id: string): Promise<null> {
  return client.post(`/traders/${id}/stop`);
}

/** 持仓列表（symbol 过滤 + 字段过滤） */
export function listPositions(params: { symbol?: string; fields?: string } = {}): Promise<Position[]> {
  return client.get('/positions', { params });
}

/**
 * 平仓（S2：pnl 必须走 body 而非 query —— 请求签名只覆盖 timestamp+path+body，
 * query 不受 HMAC 保护，放 query 可被中间人篡改平仓盈亏）。
 */
export function closePosition(id: string, pnl?: number): Promise<Position> {
  return client.delete(`/positions/${id}`, { data: pnl !== undefined ? { pnl } : {} });
}
