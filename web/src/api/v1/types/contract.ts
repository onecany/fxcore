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
  traderId: string;
  openedAt: string;
  closedAt?: string;
  // ---- §10/§15 扩展（后端新增字段） ----
  trader_id?: string;
  exchange_id?: string;
  quantity?: number;
  entry_quantity?: number;
  mark_price?: number;
  unrealized_pnl?: number;
  leverage?: number;
  status?: 'OPEN' | 'CLOSED';
  entry_time?: number;
  exit_time?: number;
  exit_price?: number;
  realized_pnl?: number;
  fee?: number;
  close_reason?: string;
  source?: string;
}

// ========== 策略模块（§3 简版 + §10 完整树） ==========
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
  exchange_type: ExchangeType;
  account_name: string;
  enabled: boolean;
  testnet?: boolean;
  /** 凭据脱敏返回（如 ab****cd）；创建/更新时传原文 */
  api_key_prefix?: string;
  // 各交易所公开字段（凭据列加密存储，响应不含原文）:
  // CEX: api_key + secret_key（okx/gate/kucoin 额外 passphrase）
  // hyperliquid: hyperliquid_wallet_addr
  // aster: aster_user + aster_signer + aster_private_key
  // lighter: lighter_wallet_addr + lighter_private_key + lighter_api_key_private_key + lighter_api_key_index
  hyperliquid_wallet_addr?: string;
  aster_user?: string;
  aster_signer?: string;
  lighter_wallet_addr?: string;
  lighter_api_key_index?: number;
  created_at: string;
  updated_at: string;
}
export interface CreateExchangeRequest {
  exchange_type: ExchangeType;
  account_name: string;
  enabled?: boolean;
  testnet?: boolean;
  api_key?: string;
  secret_key?: string;
  passphrase?: string;
  hyperliquid_wallet_addr?: string;
  aster_user?: string;
  aster_signer?: string;
  aster_private_key?: string;
  lighter_wallet_addr?: string;
  lighter_private_key?: string;
  lighter_api_key_private_key?: string;
  lighter_api_key_index?: number;
}

// ========== 策略（完整 StrategyConfig 树） ==========
export type CoinSourceType = 'static' | 'ai500' | 'oi_top' | 'oi_low' | 'mixed';
export interface CoinSourceConfig {
  source_type: CoinSourceType; static_coins?: string[]; excluded_coins?: string[];
  use_ai500: boolean; ai500_limit?: number; use_oi_top?: boolean; oi_top_limit?: number;
  use_oi_low?: boolean; oi_low_limit?: number;
}
export interface KlineConfig {
  primary_timeframe: string; primary_count: number; longer_timeframe?: string;
  longer_count?: number; enable_multi_timeframe: boolean; selected_timeframes?: string[];
}
export interface IndicatorConfig {
  klines: KlineConfig; enable_raw_klines: boolean; enable_ema: boolean; enable_macd: boolean;
  enable_rsi: boolean; enable_atr: boolean; enable_boll: boolean; enable_volume: boolean;
  enable_oi: boolean; enable_funding_rate: boolean; ema_periods?: number[]; rsi_periods?: number[];
  atr_periods?: number[]; boll_periods?: number[];
  enable_quant_data: boolean; enable_quant_oi: boolean; enable_quant_netflow: boolean;
  enable_oi_ranking: boolean; oi_ranking_duration?: string; oi_ranking_limit?: number;
  enable_netflow_ranking: boolean; enable_price_ranking: boolean;
}
export interface RiskControlConfig {
  max_positions: number; btc_eth_max_leverage: number; altcoin_max_leverage: number;
  btc_eth_max_position_value_ratio: number; altcoin_max_position_value_ratio: number;
  max_margin_usage: number; min_position_size: number; min_risk_reward_ratio: number;
  min_confidence: number;
}
export interface PromptSections {
  role_definition?: string; trading_frequency?: string; entry_standards?: string; decision_process?: string;
}
export interface GridStrategyConfig {
  symbol?: string; grid_count?: number; lower_price?: number; upper_price?: number;
  leverage?: number; investment_usd?: number; max_orders?: number;
}
export interface StrategyConfig {
  strategy_type?: string; language?: 'zh' | 'en';
  coin_source: CoinSourceConfig; indicators: IndicatorConfig; risk_control: RiskControlConfig;
  custom_prompt?: string;
  prompt_sections?: PromptSections;
  grid_config?: GridStrategyConfig | null;
}
export interface StrategyItem {
  id: string; name: string; description?: string;
  is_active: boolean; is_default?: boolean; is_public?: boolean;
  config: StrategyConfig; // 后端返回 StrategyConfig JSON
  created_at: string; updated_at: string;
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
  price?: number; stop_loss?: number; take_profit?: number; confidence?: number;
  reasoning?: string; order_id?: string; success?: boolean; error?: string;
}
export interface DecisionRecord {
  id: string; trader_id: string; cycle_number: number; timestamp: string;
  system_prompt?: string; input_prompt?: string; cot_trace?: string; decision_json?: string;
  raw_response?: string; candidate_coins?: string[]; execution_log?: string;
  decisions: DecisionAction[]; success: boolean;
  error_message?: string; ai_request_duration_ms?: number;
}
export interface Statistics {
  total_trades: number; win_rate: number; total_pnl: number; profit_factor?: number;
  sharpe_ratio?: number; max_drawdown_pct?: number; avg_win?: number; avg_loss?: number;
}

// ========== 回测 ==========
export type BacktestState = 'created' | 'running' | 'paused' | 'stopped' | 'completed' | 'failed' | 'liquidated';
export interface BacktestConfig {
  strategy_id?: string; symbols?: string[]; timeframes?: string[]; cadence?: number;
  initial_balance?: number; leverage?: number; prompt_variant?: string; prompt_template?: string;
  temperature?: number; fee_bps?: number; slippage_bps?: number; fill_policy?: 'next_open' | 'bar_vwap' | 'mid';
  start_time?: number; end_time?: number; ai_cache?: boolean; replay_only?: boolean;
}
export interface RunMetadata {
  run_id: string; state: BacktestState; label?: string;
  symbol_count?: number; progress_pct?: number; equity_last?: number;
  max_drawdown_pct?: number; liquidated?: boolean; last_error?: string; created_at: string;
}
export interface RunSummary extends RunMetadata {
  symbols?: string[]; timeframe?: string;
}
export interface BacktestStatusPayload extends RunMetadata {
  current_cycle?: number; total_cycles?: number;
}
export interface BacktestEquityPoint { timestamp: number; equity: number; available: number; pnl: number; pnl_pct: number; drawdown_pct: number; cycle: number; }
export interface BacktestTradeEvent {
  timestamp: number; symbol: string; action: string; side?: string; quantity: number; price: number;
  fee: number; slippage: number; order_value: number; realized_pnl: number; leverage: number;
  cycle: number; position_after: number; liquidation_flag?: boolean; note?: string;
}
export interface BacktestMetrics {
  total_trades: number; win_rate: number; total_pnl: number; profit_factor?: number;
  sharpe_ratio?: number; max_drawdown_pct?: number; avg_win?: number; avg_loss?: number;
  liquidated?: boolean; final_equity?: number; return_pct?: number;
}

// ========== 辩论竞技场 ==========
export type DebateStatus = 'pending' | 'running' | 'voting' | 'completed' | 'cancelled';
export type DebatePersonality = 'bull' | 'bear' | 'analyst' | 'contrarian' | 'risk_manager';
export interface DebateSession {
  id: string; name: string; strategy_id: string; status: DebateStatus; symbol: string;
  max_rounds: number; current_round: number; interval_minutes: number; prompt_variant: string;
  auto_execute: boolean; trader_id?: string; created_at: string; updated_at: string;
}
export interface DebateParticipant {
  id: string; session_id: string; ai_model_id: string; ai_model_name: string;
  provider: string; personality: DebatePersonality; color: string; speak_order: number;
}
export interface DebateMessage {
  id: string; session_id: string; round: number; participant_id?: string;
  personality?: string; ai_model_name?: string; content: string; timestamp: number;
}
export interface DebateVote {
  session_id: string; ai_model_id: string; action: DecisionActionType; symbol: string;
  confidence: number; leverage: number; position_pct: number; stop_loss_pct: number;
  take_profit_pct: number; reasoning?: string;
}
export interface CreateDebateRequest {
  name: string; strategy_id: string; symbol: string;
  participants: string[]; // AI model IDs（≥2）
  max_rounds: number; interval_minutes?: number; prompt_variant?: string;
  auto_execute?: boolean; trader_id?: string;
}
export interface SessionWithDetails extends DebateSession {
  participants: DebateParticipant[]; messages: DebateMessage[]; votes: DebateVote[];
}

// ========== Telegram ==========
export interface TelegramConfig { bot_token: string; model_id: string; chat_id?: string; }
export interface TelegramConfigDTO {
  bot_token_prefix?: string; chat_id?: string; username?: string;
  bound_at?: number; model_id?: string; language?: string;
}

// ========== 加密工具 ==========
export interface PublicKeyDTO { public_key: string; algorithm: string; }
export interface DecryptRequest { wrapped_key: string; iv: string; ciphertext: string; aad: string; ts: string; }
export interface DecryptResponse { plaintext: string; }

// ========== 数据查询 ==========
export interface StatusDTO { is_running: boolean; trader_id: string; }
export interface AccountInfoDTO {
  total_equity: number; available_balance: number; unrealized_pnl?: number;
  position_count?: number; margin_used_pct?: number; currency: string;
}
export interface EquityPointDTO {
  timestamp: number; equity: number; balance?: number; pnl?: number; pnl_pct?: number; drawdown_pct?: number;
}
export interface OrderDTO {
  id: string; trader_id: string; exchange_id?: string; exchange_order_id?: string; client_order_id?: string;
  symbol: string; side: string; position_side?: string; type: string; time_in_force?: string;
  quantity: number; price?: number; stop_price?: number; status: string;
  filled_quantity?: number; avg_fill_price?: number; commission?: number; commission_asset?: string;
  leverage?: number; reduce_only?: boolean; created_at: string;
}
export interface FillDTO {
  id: string; trader_id: string; order_id: string; exchange_trade_id?: string;
  symbol: string; side: string; price: number; quantity: number; quote_quantity?: number;
  commission?: number; commission_asset?: string; realized_pnl?: number; is_maker?: boolean; created_at: string;
}
export interface KlineDTO {
  timestamp: number; open: number; high: number; low: number; close: number;
  volume: number; oi?: number; funding?: number;
}
