// FXcore-web API v1 交易所账户模块（§10 exchanges）。
// 对应后端 GET/POST/PUT/DELETE /exchanges 与 GET /crypto/public-key。
import client from '../client';
import type { ExchangeAccount, CreateExchangeRequest, ApiResponse } from '../types/contract';

/** 交易所类型字典（本地常量，对齐后端 exchange_svc 校验；动态表单渲染用） */
export interface ExchangeTypeOption {
  type: string;
  name: string;
  cex: boolean;
  requiresPassphrase: boolean;
  fields: string[];
}

const EXCHANGE_TYPES: ExchangeTypeOption[] = [
  { type: 'binance', name: 'Binance', cex: true, requiresPassphrase: false, fields: ['api_key', 'secret_key'] },
  { type: 'bybit', name: 'Bybit', cex: true, requiresPassphrase: false, fields: ['api_key', 'secret_key'] },
  { type: 'okx', name: 'OKX', cex: true, requiresPassphrase: true, fields: ['api_key', 'secret_key', 'passphrase'] },
  { type: 'bitget', name: 'Bitget', cex: true, requiresPassphrase: true, fields: ['api_key', 'secret_key', 'passphrase'] },
  { type: 'gate', name: 'Gate.io', cex: true, requiresPassphrase: false, fields: ['api_key', 'secret_key'] },
  { type: 'kucoin', name: 'KuCoin', cex: true, requiresPassphrase: true, fields: ['api_key', 'secret_key', 'passphrase'] },
  { type: 'indodax', name: 'Indodax', cex: true, requiresPassphrase: false, fields: ['api_key', 'secret_key'] },
  { type: 'hyperliquid', name: 'Hyperliquid', cex: false, requiresPassphrase: false, fields: ['hyperliquid_wallet_addr', 'hyperliquid_private_key'] },
  { type: 'aster', name: 'Aster', cex: false, requiresPassphrase: false, fields: ['aster_user', 'aster_signer', 'aster_private_key'] },
  { type: 'lighter', name: 'Lighter', cex: false, requiresPassphrase: false, fields: ['lighter_wallet_addr', 'lighter_private_key', 'lighter_api_key_private_key', 'lighter_api_key_index'] },
];

/** 交易所类型字典（本地常量） */
export function listExchangeTypes(): Promise<ExchangeTypeOption[]> {
  return Promise.resolve(EXCHANGE_TYPES);
}

/** 账户列表 */
export function listExchanges(): Promise<ExchangeAccount[]> {
  return client.get<ExchangeAccount[]>('/exchanges');
}

/** 创建账户（凭据后端 RSA 加密存储，响应零泄漏） */
export function createExchange(req: CreateExchangeRequest): Promise<ExchangeAccount> {
  return client.post<ExchangeAccount>('/exchanges', req);
}

/** 更新账户（传新 key 触发 Key 轮换；字段为指针语义，未传保留） */
export function updateExchange(id: string, req: Partial<CreateExchangeRequest>): Promise<ExchangeAccount> {
  return client.put<ExchangeAccount>(`/exchanges/${id}`, req);
}

/** 删除账户（软删：断开关联） */
export function deleteExchange(id: string): Promise<null> {
  return client.delete<null>(`/exchanges/${id}`);
}

/** 切换启用状态 */
export function setExchangeEnabled(id: string, enabled: boolean): Promise<ExchangeAccount> {
  return client.patch<ExchangeAccount>(`/exchanges/${id}`, { enabled });
}

/** RSA-2048 公钥（前端加密敏感字段的备选通道） */
export function getRsaKey(): Promise<ApiResponse<{ publicKey: string }>> {
  return client.get<ApiResponse<{ publicKey: string }>>('/crypto/public-key');
}
