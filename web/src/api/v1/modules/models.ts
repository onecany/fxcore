// FXcore-web API v1 模型管理模块（CRUD + 测试连接）。
// 对应后端 GET/POST/PUT/DELETE /models 与 POST /models/{id}/test。
import client from '../client';
import type { AIModel, CreateModelRequest, AIProvider } from '../types/contract';

// GET /models/providers 动态下拉
export interface ProviderOption {
  provider: AIProvider | string;
  models: string[];
}

/** 模型列表（支持 ?fields= 字段过滤） */
export function listModels(fields?: string): Promise<AIModel[]> {
  return client.get<AIModel[]>('/models', { params: fields ? { fields } : {} });
}

/** 提供商 + 模型下拉 */
export function listProviders(): Promise<ProviderOption[]> {
  return client.get<ProviderOption[]>('/models/providers');
}

/** 创建模型（apiKey 后端 RSA 加密存储） */
export function createModel(req: CreateModelRequest): Promise<AIModel> {
  return client.post<AIModel>('/models', req);
}

/** 更新模型（传新 apiKey 触发 Key 轮换） */
export function updateModel(id: string, req: CreateModelRequest): Promise<AIModel> {
  return client.put<AIModel>(`/models/${id}`, req);
}

/** 软删除模型 */
export function deleteModel(id: string): Promise<null> {
  return client.delete<null>(`/models/${id}`);
}

/** 连通测试：body 可传 apiKey 覆盖，留空则用已存 Key */
export function testModel(
  id: string,
  req?: Partial<{ apiKey: string; modelName: string; provider: AIProvider }>,
): Promise<{ success: boolean; latency: number; error?: string }> {
  return client.post(`/models/${id}/test`, req ?? {});
}
