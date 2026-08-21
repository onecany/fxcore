// FXcore-web API v1 回测模块（§11 backtest）。
// 对应后端 /backtest/* 15 端点。
import client from '../client';
import type {
  BacktestConfig,
  RunMetadata,
  RunSummary,
  BacktestStatusPayload,
  BacktestEquityPoint,
  BacktestTradeEvent,
  BacktestMetrics,
  DecisionRecord,
  PaginatedData,
} from '../types/contract';

export type BacktestControlAction = 'pause' | 'resume' | 'stop';

/** 启动回测（后端 body 需 config 包装） */
export function startBacktest(req: BacktestConfig): Promise<RunMetadata> {
  return client.post<RunMetadata>('/backtest/start', { config: req });
}

/** 控制：pause | resume | stop */
export function controlBacktest(runId: string, action: BacktestControlAction): Promise<RunMetadata> {
  return client.post<RunMetadata>(`/backtest/${action}`, { run_id: runId });
}

/** 打标签 */
export function labelBacktest(runId: string, label: string): Promise<RunMetadata> {
  return client.post<RunMetadata>('/backtest/label', { run_id: runId, label });
}

/** 删除（含关联数据） */
export function deleteBacktest(runId: string): Promise<null> {
  return client.post<null>('/backtest/delete', { run_id: runId });
}

/** 运行状态（含运行中进度） */
export function backtestStatus(runId: string): Promise<BacktestStatusPayload> {
  return client.get<BacktestStatusPayload>('/backtest/status', { params: { run_id: runId } });
}

/** 运行列表（状态过滤 + 搜索 + 分页） */
export function listBacktestRuns(params?: {
  state?: string;
  search?: string;
  page?: number;
  size?: number;
}): Promise<{ items: RunSummary[]; total: number }> {
  return client.get('/backtest/runs', { params });
}

/** 权益曲线 */
export function backtestEquity(runId: string, tf?: string, limit?: number): Promise<BacktestEquityPoint[]> {
  return client.get<BacktestEquityPoint[]>('/backtest/equity', {
    params: { run_id: runId, tf, limit },
  });
}

/** 成交明细 */
export function backtestTrades(runId: string): Promise<BacktestTradeEvent[]> {
  return client.get<BacktestTradeEvent[]>('/backtest/trades', { params: { run_id: runId } });
}

/** 绩效指标（未就绪时后端 202 + data:null → client 解信封为 null；就绪返回 metrics 本体） */
export function backtestMetrics(runId: string): Promise<BacktestMetrics | null> {
  return client.get('/backtest/metrics', { params: { run_id: runId } });
}

/** 单周期决策追踪 */
export function backtestTrace(runId: string, cycle: number): Promise<DecisionRecord> {
  return client.get<DecisionRecord>('/backtest/trace', { params: { run_id: runId, cycle } });
}

/** 决策记录（分页信封 {items, pagination}） */
export function backtestDecisions(runId: string, page?: number, size?: number): Promise<PaginatedData<DecisionRecord>> {
  return client.get('/backtest/decisions', { params: { run_id: runId, page, size } });
}

/** 导出 zip（决策 + 成交 + 权益 JSON） */
export function exportBacktest(runId: string): Promise<Blob> {
  return client.get<Blob>('/backtest/export', { params: { run_id: runId }, responseType: 'blob' });
}

/** K 线（回测区间数据源链） */
export function backtestKlines(runId: string, symbol: string, timeframe?: string): Promise<unknown[]> {
  return client.get('/backtest/klines', { params: { run_id: runId, symbol, timeframe } });
}
