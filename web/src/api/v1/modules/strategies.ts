// FXcore-web API v1 策略模块（§10 strategies）。
// 对应后端 GET/POST/PUT/DELETE /strategies、activate、preview-prompt、test-run。
import client from '../client';
import type { StrategyItem, StrategyConfig, CreateStrategyRequest, UpdateStrategyRequest } from '../types/contract';

/** 策略列表（分页信封） */
export function listStrategies(): Promise<{ items: StrategyItem[]; pagination: { total: number } }> {
  return client.get('/strategies');
}

/** 创建策略 */
export function createStrategy(req: CreateStrategyRequest): Promise<StrategyItem> {
  return client.post<StrategyItem>('/strategies', req);
}

/** 更新策略（顶层字段部分合并，未提及字段保留） */
export function updateStrategy(id: string, req: UpdateStrategyRequest): Promise<StrategyItem> {
  return client.put<StrategyItem>(`/strategies/${id}`, req);
}

/** 删除策略（运行中占用拒绝） */
export function deleteStrategy(id: string): Promise<null> {
  return client.delete<null>(`/strategies/${id}`);
}

/** 激活策略（全局唯一生效） */
export function activateStrategy(id: string): Promise<StrategyItem> {
  return client.post<StrategyItem>(`/strategies/${id}/activate`);
}

/** 预览系统提示词（POST /strategies/preview-prompt，body 传 config；lint 警告面板后续接入） */
export function previewPrompt(config: StrategyConfig): Promise<{ prompt: string }> {
  return client.post('/strategies/preview-prompt', { config });
}

/** 试跑（空骨架：策略可用性验证） */
export function testRun(id: string): Promise<{ success: boolean; message: string }> {
  return client.post(`/strategies/${id}/test-run`);
}

/** 默认配置（新建表单预填） */
export function defaultConfig(): Promise<StrategyConfig> {
  return client.get<StrategyConfig>('/strategies/default-config');
}
