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
export type Exchange = 'binance' | 'bybit' | 'okx' | 'bitget' | 'gate' | 'kucoin' | 'hyperliquid' | 'aster' | 'indodax' | 'lighter';
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
  metrics: { totalPnl: number; winRate: number; tradeCount: number; dailyPnl: number; };
  createdAt: string; updatedAt: string;
}

// ========== 持仓模块（DashboardSummary 引用 + §10 扩展） ==========
// 说明：后端 model.Position 保留旧字段（size/pnl/opened_at/closed_at）并新增
// §10/§15 契约字段（quantity/leverage/status/entry_time/exit_time/realized_pnl/
// fee/close_reason）。前端此处同时保留新旧两组（新字段可选，兼容 dashboard 旧引用）。
export interface Position {
  id: string;
  symbol: string;
  side: 'long' | 'short';
  size: number;
  entryPrice: number;
  pnl: number;
  traderId?: string;
  openedAt: string;
  closedAt?: string;
  // ---- §10/§15 扩展（后端新增字段） ----
  exchangeId?: string;
  quantity?: number;
  entryQuantity?: number;
  markPrice?: number;
  // 注意：unrealized_pn_l/realized_pn_l 归一化后是 unrealizedPnL/realizedPnL（大写 L）
  unrealizedPnL?: number;
  // /positions/history 裸 model 直出是 realized_pnl（标准 snake）→ toCamel 得 realizedPnl（小写 l）。
  // 与 DTO 风格 realizedPnL（大写 L）并存——读取处双拼写兼容（pn_l 契约坑，见 fxcore-development skill）。
  realizedPnl?: number;
  leverage?: number;
  status?: 'OPEN' | 'CLOSED';
  entryTime?: number;
  exitTime?: number;
  exitPrice?: number;
  realizedPnL?: number;
  fee?: number;
  closeReason?: string;
  source?: string;
}

// ========== 策略模块（§3 简版 + §10 完整树） ==========
export interface Strategy {
  id: string; name: string; description: string; type: 'grid' | 'dca' | 'ai'; config: Record<string, any>; isActive: boolean;
}

// ========== 仪表盘聚合接口（减少轮询，合并请求） ==========
export interface DashboardSummary {
  account: { totalBalance: number; totalPnl: number; dailyPnl: number; };
  activeTraders: TraderResponse[];   // 只返回运行中的前5个
  recentPositions: Position[];       // 只返回最新的5条
  systemHealth: { status: 'healthy' | 'degraded'; aiLatency: number; exchangeLatency: number; };
}

// ════════════════════════════════════════════════════════════
// §10 扩展数据契约（交易所/策略/持仓/决策/回测/辩论/Telegram）
// 与 internal/api/v1/dto/ 后端结构体对齐（snake_case，后端权威）。
// 命名偏差：§10 文档的 Exchange 账户接口与 §3 交易员模块的
// Exchange union 类型同名冲突，此处账户接口命名为 ExchangeAccount。
// ════════════════════════════════════════════════════════════

// ========== 交易所（10 家，扩展原 5 家） ==========
export type ExchangeType =
  | 'binance' | 'bybit' | 'okx' | 'bitget' | 'gate' | 'kucoin' | 'indodax'   // CEX
  | 'hyperliquid' | 'aster' | 'lighter';                                        // DEX

export interface ExchangeAccount {
  id: string;
  exchangeType: ExchangeType;
  accountName: string;
  enabled: boolean;
  testnet?: boolean;
  /** 凭据脱敏返回（如 ab****cd）；创建/更新时传原文 */
  apiKeyPrefix?: string;
  // 各交易所公开字段（凭据列加密存储，响应不含原文）:
  // CEX: api_key + secret_key（okx/gate/kucoin 额外 passphrase）
  // hyperliquid: hyperliquid_wallet_addr
  // aster: aster_user + aster_signer + aster_private_key
  // lighter: lighter_wallet_addr + lighter_private_key + lighter_api_key_private_key + lighter_api_key_index
  hyperliquidWalletAddr?: string;
  asterUser?: string;
  asterSigner?: string;
  lighterWalletAddr?: string;
  lighterApiKeyIndex?: number;
  createdAt: string;
  updatedAt: string;
}
export interface CreateExchangeRequest {
  exchangeType: ExchangeType;
  accountName: string;
  enabled?: boolean;
  testnet?: boolean;
  apiKey?: string;
  secretKey?: string;
  passphrase?: string;
  hyperliquidWalletAddr?: string;
  asterUser?: string;
  asterSigner?: string;
  asterPrivateKey?: string;
  lighterWalletAddr?: string;
  lighterPrivateKey?: string;
  lighterApiKeyPrivateKey?: string;
  lighterApiKeyIndex?: number;
}

// ========== 策略（完整 StrategyConfig 树） ==========
export type CoinSourceType = 'static' | 'ai500' | 'oi_top' | 'oi_low' | 'mixed';
export interface CoinSourceConfig {
  sourceType: CoinSourceType; staticCoins?: string[]; excludedCoins?: string[];
  useAi500: boolean; ai500Limit?: number; useOiTop?: boolean; oiTopLimit?: number;
  useOiLow?: boolean; oiLowLimit?: number;
}
export interface KlineConfig {
  primaryTimeframe: string; primaryCount: number; longerTimeframe?: string;
  longerCount?: number; enableMultiTimeframe: boolean; selectedTimeframes?: string[];
}
export interface IndicatorConfig {
  klines: KlineConfig; enableRawKlines: boolean; enableEma: boolean; enableMacd: boolean;
  enableRsi: boolean; enableAtr: boolean; enableBoll: boolean; enableVolume: boolean;
  enableOi: boolean; enableFundingRate: boolean; emaPeriods?: number[]; rsiPeriods?: number[];
  atrPeriods?: number[]; bollPeriods?: number[];
  enableQuantData: boolean; enableQuantOi: boolean; enableQuantNetflow: boolean;
  enableOiRanking: boolean; oiRankingDuration?: string; oiRankingLimit?: number;
  enableNetflowRanking: boolean; enablePriceRanking: boolean;
}
export interface RiskControlConfig {
  maxPositions: number; btcEthMaxLeverage: number; altcoinMaxLeverage: number;
  btcEthMaxPositionValueRatio: number; altcoinMaxPositionValueRatio: number;
  maxMarginUsage: number; minPositionSize: number; minRiskRewardRatio: number;
  minConfidence: number;
}
export interface PromptSections {
  roleDefinition?: string; tradingFrequency?: string; entryStandards?: string; decisionProcess?: string;
}
export interface GridStrategyConfig {
  symbol?: string; gridCount?: number; lowerPrice?: number; upperPrice?: number;
  leverage?: number; investmentUsd?: number; maxOrders?: number;
}
export interface StrategyConfig {
  strategyType?: string; language?: 'zh' | 'en';
  coinSource: CoinSourceConfig; indicators: IndicatorConfig; riskControl: RiskControlConfig;
  customPrompt?: string;
  promptSections?: PromptSections;
  gridConfig?: GridStrategyConfig | null;
}
export interface StrategyItem {
  id: string; name: string; description?: string;
  isActive: boolean; isDefault?: boolean; isPublic?: boolean;
  config: StrategyConfig; // 后端返回 StrategyConfig JSON
  createdAt: string; updatedAt: string;
}
export interface CreateStrategyRequest {
  name: string; description?: string; lang?: 'zh' | 'en'; config?: Partial<StrategyConfig>;
}
export interface UpdateStrategyRequest {
  name?: string; description?: string; config?: Partial<StrategyConfig>;
}

// ========== 决策 / 统计 ==========
export type DecisionActionType = 'open_long' | 'open_short' | 'close_long' | 'close_short' | 'hold' | 'wait';
export interface DecisionAction {
  action: DecisionActionType; symbol: string; quantity?: number; leverage?: number;
  price?: number; stopLoss?: number; takeProfit?: number; confidence?: number;
  reasoning?: string; orderId?: string; success?: boolean; error?: string;
}
export interface DecisionRecord {
  id: string; traderId: string; cycleNumber: number; timestamp: string;
  systemPrompt?: string; inputPrompt?: string; cotTrace?: string; decisionJson?: string;
  rawResponse?: string; candidateCoins?: string[]; executionLog?: string;
  decisions: DecisionAction[]; success: boolean;
  errorMessage?: string; aiRequestDurationMs?: number;
}

/** AI 试跑结果（POST /strategies/test-run；解析失败时 parsed=false + raw 原始输出保留） */
export interface TestRunResult {
  prompt: string;
  raw: string;
  // 解析失败时后端可能返回 null（防御：前端用 ?? [] 兜底）
  decisions?: DecisionAction[];
  parsed: boolean;
  latencyMs: number;
  error?: string;
}

// ========== §15 交易运行时数据 ==========
/** 权益点（后端 EquityPointDTO：timestamp=Unix秒） */
export interface EquitySnapshot {
  timestamp: number; equity: number; balance: number;
  pnl?: number; drawdownPct?: number;
  marginUsedPct?: number; positionCount?: number;
}
export interface Order {
  id: string; traderId: string; exchangeId?: string; exchangeOrderId?: string; clientOrderId?: string;
  symbol: string; side: 'buy' | 'sell'; positionSide?: 'long' | 'short';
  type: string; timeInForce?: string; quantity: number; price?: number; stopPrice?: number;
  status: string; filledQuantity?: number; avgFillPrice?: number;
  commission?: number; commissionAsset?: string; leverage?: number;
  reduceOnly?: boolean; closePosition?: boolean; createdAt: string; updatedAt?: string;
}
export interface Fill {
  id: string; traderId: string; orderId: string; exchangeOrderId?: string; exchangeTradeId?: string;
  symbol: string; side: 'buy' | 'sell'; price: number; quantity: number; quoteQuantity?: number;
  commission?: number; commissionAsset?: string; realizedPnl?: number; isMaker?: boolean; createdAt: string;
}
export interface Statistics {
  totalTrades: number; winRate: number; totalPnl: number; profitFactor?: number;
  sharpeRatio?: number; maxDrawdownPct?: number; avgWin?: number; avgLoss?: number;
}

// ========== 回测 ==========
export type BacktestState = 'created' | 'running' | 'paused' | 'stopped' | 'completed' | 'failed' | 'liquidated';
export interface BacktestConfig {
  strategyId?: string; symbols?: string[]; timeframes?: string[]; cadence?: number;
  initialBalance?: number; leverage?: number; promptVariant?: string; promptTemplate?: string;
  temperature?: number; feeBps?: number; slippageBps?: number; fillPolicy?: 'next_open' | 'bar_vwap' | 'mid';
  startTime?: number; endTime?: number; aiCache?: boolean; replayOnly?: boolean;
}
export interface RunMetadata {
  runId: string; state: BacktestState; label?: string;
  symbolCount?: number; progressPct?: number; equityLast?: number;
  maxDrawdownPct?: number; liquidated?: boolean; lastError?: string; createdAt: string;
}
export interface RunSummary extends RunMetadata {
  symbols?: string[]; timeframe?: string;
}
export interface BacktestStatusPayload extends RunMetadata {
  currentCycle?: number; totalCycles?: number;
}
export interface BacktestEquityPoint { timestamp: number; equity: number; available: number; pnl: number; pnlPct: number; drawdownPct: number; cycle: number; }
export interface BacktestTradeEvent {
  timestamp: number; symbol: string; action: string; side?: string; quantity: number; price: number;
  fee: number; slippage: number; orderValue: number; realizedPnl: number; leverage: number;
  cycle: number; positionAfter: number; liquidationFlag?: boolean; note?: string;
}
export interface BacktestMetrics {
  totalTrades: number; winRate: number; totalPnl: number; profitFactor?: number;
  sharpeRatio?: number; maxDrawdownPct?: number; avgWin?: number; avgLoss?: number;
  liquidated?: boolean; finalEquity?: number; returnPct?: number;
}

// ========== 辩论竞技场 ==========
export type DebateStatus = 'pending' | 'running' | 'voting' | 'completed' | 'cancelled';
export type DebatePersonality = 'bull' | 'bear' | 'analyst' | 'contrarian' | 'risk_manager';
export interface DebateSession {
  id: string; name: string; strategyId: string; status: DebateStatus; symbol: string;
  maxRounds: number; currentRound: number; intervalMinutes: number; promptVariant: string;
  autoExecute: boolean; traderId?: string; createdAt: string; updatedAt: string;
}
export interface DebateParticipant {
  id: string; sessionId: string; aiModelId: string; aiModelName: string;
  provider: string; personality: DebatePersonality; color: string; speakOrder: number;
}
export interface DebateMessage {
  id: string; sessionId: string; round: number; participantId?: string;
  personality?: string; aiModelName?: string; content: string; timestamp: number;
}
export interface DebateVote {
  sessionId: string; aiModelId: string; action: DecisionActionType; symbol: string;
  confidence: number; leverage: number; positionPct: number; stopLossPct: number;
  takeProfitPct: number; reasoning?: string;
}
export interface CreateDebateRequest {
  name: string; strategyId: string; symbol: string;
  participants: string[]; // AI model IDs（≥2）
  maxRounds: number; intervalMinutes?: number; promptVariant?: string;
  autoExecute?: boolean; traderId?: string;
}
export interface SessionWithDetails extends DebateSession {
  participants: DebateParticipant[]; messages: DebateMessage[]; votes: DebateVote[];
}

// ========== Telegram ==========
export interface TelegramConfig { botToken: string; modelId: string; chatId?: string; }
export interface TelegramConfigDTO {
  botTokenPrefix?: string; chatId?: string; username?: string;
  boundAt?: number; modelId?: string; language?: string;
}

// ========== 加密工具 ==========
export interface PublicKeyDTO { publicKey: string; algorithm: string; }
export interface DecryptRequest { wrappedKey: string; iv: string; ciphertext: string; aad: string; ts: string; }
export interface DecryptResponse { plaintext: string; }

// ========== 数据查询 ==========
export interface StatusDTO { isRunning: boolean; traderId: string; }
export interface AccountInfoDTO {
  totalEquity: number; availableBalance: number; unrealizedPnl?: number;
  positionCount?: number; marginUsedPct?: number; currency: string;
}
export interface EquityPointDTO {
  timestamp: number; equity: number; balance?: number; pnl?: number; pnlPct?: number; drawdownPct?: number;
}
export interface OrderDTO {
  id: string; traderId: string; exchangeId?: string; exchangeOrderId?: string; clientOrderId?: string;
  symbol: string; side: string; positionSide?: string; type: string; timeInForce?: string;
  quantity: number; price?: number; stopPrice?: number; status: string;
  filledQuantity?: number; avgFillPrice?: number; commission?: number; commissionAsset?: string;
  leverage?: number; reduceOnly?: boolean; createdAt: string;
}
export interface FillDTO {
  id: string; traderId: string; orderId: string; exchangeTradeId?: string;
  symbol: string; side: string; price: number; quantity: number; quoteQuantity?: number;
  commission?: number; commissionAsset?: string; realizedPnl?: number; isMaker?: boolean; createdAt: string;
}
export interface KlineDTO {
  timestamp: number; open: number; high: number; low: number; close: number;
  volume: number; oi?: number; funding?: number;
}
