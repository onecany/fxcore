// FXcore-web API v1 仪表盘模块（3 合 1 聚合接口，减少轮询）。
import client from '../client';
import type { DashboardSummary } from '../types/contract';

/** 获取仪表盘聚合数据（后端 sync.WaitGroup 并发聚合） */
export function getDashboard(): Promise<DashboardSummary> {
  return client.get<DashboardSummary>('/dashboard');
}
