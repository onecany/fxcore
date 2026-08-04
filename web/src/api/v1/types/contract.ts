// ============================================================
// FXcore-web API v1 数据契约（TypeScript 黄金标准）
// 与 internal/api/v1/dto/ 后端结构体对齐（字段语义一一对应）
//
// ⚠️ 已知偏差（API设计.md 内部矛盾，见 data/API设计.md 复盘）：
//   1. 文档第3节要求本契约 camelCase，同时第9节要求 Go JSON Tag 为
//      snake_case。当前按文档字面执行：后端响应为 snake_case，
//      前端类型为 camelCase，集成时需在 client.ts 归一化。
//   2. Position 在 DashboardSummary 中被引用但文档未定义，此处补齐。
//   3. LoginResponse.signSecret 为扩展字段（文档 6.1 请求签名需要
//      user_secret，仅登录时下发一次）。
// ============================================================

// ========== 通用契约 ==========
export interface ApiResponse<T = any> {
  code: number;
  message: string;
  data: T;
  timestamp: number;
  requestId: string;
}

export interface PaginatedData<T> {
  items: T[];
  pagination: { page: number; pageSize: number; total: number; totalPages: number };
}

// ========== 字段过滤支持（所有GET列表接口通用） ==========
export interface FieldsQuery { fields?: string; } // 例: ?fields=id,name,status

// ========== 认证模块 ==========
export interface LoginRequest { email: string; password: string; }
export interface LoginResponse {
  user: User;
  accessToken: string;
  /** 扩展字段：请求签名所需 user_secret（HMAC-SHA256），仅登录时下发一次，请存内存勿落盘 */
  signSecret?: string;
  /** 扩展字段：Refresh 轮换闭环所需（文档契约未定义，见 data/API设计.md 复盘） */
  refreshToken?: string;
}
export interface RefreshRequest { refreshToken: string; }
export interface User { id: string; email: string; nickname: string; avatar?: string; }

// ========== AI模型管理（全面增强） ==========
export type AIProvider = 'deepseek' | 'qwen' | 'claude' | 'gpt' | 'gemini' | 'custom';
export interface AIModel {
  id: string;
  name: string;                // 用户自定义别名
  provider: AIProvider;
  modelName: string;           // 实际模型ID，如 gpt-4-turbo
  apiKeyPrefix: string;        // 脱敏显示，如 'sk-****abcd'
  status: 'active' | 'inactive' | 'error';
  config: Record<string, any>; // 温度、TopP等
  createdAt: string;
  lastTestAt?: string;
}
export interface CreateModelRequest {
  name: string; provider: AIProvider; modelName: string; apiKey: string; config?: Record<string, any>;
}
export interface TestModelRequest { apiKey: string; modelName: string; provider: AIProvider; }

// ========== 交易员模块 ==========
export type Exchange = 'binance' | 'hyperliquid' | 'aster' | 'bybit' | 'okx';
export type TraderStatus = 'idle' | 'running' | 'paused' | 'stopped' | 'error';
export interface RiskConfig { maxPositionSize: number; stopLoss: number; takeProfit: number; maxDailyLoss: number; }
export interface CreateTraderRequest {
  name: string; exchange: Exchange;
  modelConfig: { provider: AIProvider; modelId: string; parameters?: Record<string, any>; }; // 引用AI模型ID
  strategyId: string;
  riskConfig: RiskConfig;
  schedule?: { interval: number; activeHours?: { start: string; end: string }[]; };
}
export interface TraderResponse extends CreateTraderRequest {
  id: string; status: TraderStatus;
  metrics: { totalPnL: number; winRate: number; tradeCount: number; dailyPnL: number; };
  createdAt: string; updatedAt: string;
}

// ========== 持仓模块（DashboardSummary 引用，文档未定义，此处补齐） ==========
export interface Position {
  id: string;
  symbol: string;
  side: 'long' | 'short';
  size: number;
  entryPrice: number;
  pnl: number;
  traderId: string;
  openedAt: string;
  closedAt?: string;
}

// ========== 策略模块 ==========
export interface Strategy {
  id: string; name: string; description: string; type: 'grid' | 'dca' | 'ai'; config: Record<string, any>; isActive: boolean;
}

// ========== 仪表盘聚合接口（减少轮询，合并请求） ==========
export interface DashboardSummary {
  account: { totalBalance: number; totalPnL: number; dailyPnL: number; };
  activeTraders: TraderResponse[];   // 只返回运行中的前5个
  recentPositions: Position[];       // 只返回最新的5条
  systemHealth: { status: 'healthy' | 'degraded'; aiLatency: number; exchangeLatency: number; };
}
