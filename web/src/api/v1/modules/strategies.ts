// FXcore-web API v1 策略模块（§10 strategies）。
// 对应后端 GET/POST/PUT/DELETE /strategies、activate、preview-prompt、test-run。
import client from '../client';
import type { StrategyItem, StrategyConfig, CreateStrategyRequest, UpdateStrategyRequest, TestRunResult, LintResponse } from '../types/contract';

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

/** AI 试跑：用指定模型跑当前配置构建的提示词，返回原始输出 + 解析决策（POST /strategies/test-run） */
export function testRun(req: { config: StrategyConfig; modelId: string }): Promise<TestRunResult> {
  return client.post('/strategies/test-run', { config: req.config, model_id: req.modelId });
}

/** 默认配置（新建表单预填） */
export function defaultConfig(): Promise<StrategyConfig> {
  return client.get<StrategyConfig>('/strategies/default-config');
}

/** 配置静态检查（POST /strategies/lint，与 preview-prompt 同构：先合并默认值再 lint） */
export function lintConfig(config: Partial<StrategyConfig>): Promise<LintResponse> {
  return client.post<LintResponse>('/strategies/lint', { config });
}
