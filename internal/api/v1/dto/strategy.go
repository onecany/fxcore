package dto

import (
	"encoding/json"
	"time"
)

// ========== 策略模块（API设计.md §10 StrategyConfig 树） ==========

// CoinSourceType 币种来源类型。
type CoinSourceType string

const (
	CoinSourceStatic CoinSourceType = "static"
	CoinSourceAI500  CoinSourceType = "ai500"
	CoinSourceOITop  CoinSourceType = "oi_top"
	CoinSourceOILow  CoinSourceType = "oi_low"
	CoinSourceMixed  CoinSourceType = "mixed"
)

// CoinSourceConfig 候选币来源配置。
type CoinSourceConfig struct {
	SourceType    CoinSourceType `json:"source_type"`
	StaticCoins   []string       `json:"static_coins,omitempty"`
	ExcludedCoins []string       `json:"excluded_coins,omitempty"`
	UseAI500      bool           `json:"use_ai500"`
	AI500Limit    *int           `json:"ai500_limit,omitempty"`
	UseOITop      bool           `json:"use_oi_top"`
	OITopLimit    *int           `json:"oi_top_limit,omitempty"`
	UseOILow      bool           `json:"use_oi_low"`
	OILowLimit    *int           `json:"oi_low_limit,omitempty"`
}

// KlineConfig K 线配置。
type KlineConfig struct {
	PrimaryTimeframe     string   `json:"primary_timeframe"` // 如 "15m"
	PrimaryCount         int      `json:"primary_count"`
	LongerTimeframe      string   `json:"longer_timeframe,omitempty"`
	LongerCount          int      `json:"longer_count,omitempty"`
	EnableMultiTimeframe bool     `json:"enable_multi_timeframe"`
	SelectedTimeframes   []string `json:"selected_timeframes,omitempty"`
}

// IndicatorConfig 指标配置。
type IndicatorConfig struct {
	Klines             KlineConfig `json:"klines"`
	EnableRawKlines    bool        `json:"enable_raw_klines"`
	EnableEMA          bool        `json:"enable_ema"`
	EnableMACD         bool        `json:"enable_macd"`
	EnableRSI          bool        `json:"enable_rsi"`
	EnableATR          bool        `json:"enable_atr"`
	EnableBoll         bool        `json:"enable_boll"`
	EnableVolume       bool        `json:"enable_volume"`
	EnableOI           bool        `json:"enable_oi"`
	EnableFundingRate  bool        `json:"enable_funding_rate"`
	EMAPeriods         []int       `json:"ema_periods,omitempty"`
	RSIPeriods         []int       `json:"rsi_periods,omitempty"`
	ATRPeriods         []int       `json:"atr_periods,omitempty"`
	BollPeriods        []int       `json:"boll_periods,omitempty"`
	EnableQuantData    bool        `json:"enable_quant_data"`
	EnableQuantOI      bool        `json:"enable_quant_oi"`
	EnableQuantNetflow bool        `json:"enable_quant_netflow"`
	EnableOIRanking    bool        `json:"enable_oi_ranking"`
	OIRankingDuration  string      `json:"oi_ranking_duration,omitempty"`
	OIRankingLimit     int         `json:"oi_ranking_limit,omitempty"`
	EnableNetflowRanking bool      `json:"enable_netflow_ranking"`
	EnablePriceRanking bool        `json:"enable_price_ranking"`
}

// RiskControlConfig 风控配置（§14.1 公式：保证金 1.01/lev+0.001，超限按 98% 缩减）。
type RiskControlConfig struct {
	MaxPositions                  int     `json:"max_positions"` // 默认 3
	BTCEthMaxLeverage             int     `json:"btc_eth_max_leverage"`
	AltcoinMaxLeverage            int     `json:"altcoin_max_leverage"`
	BTCEthMaxPositionValueRatio   float64 `json:"btc_eth_max_position_value_ratio"` // 仓值 ≤ ratio×equity
	AltcoinMaxPositionValueRatio  float64 `json:"altcoin_max_position_value_ratio"`
	MaxMarginUsage                float64 `json:"max_margin_usage"` // 保证金占用 ≤ 此比例
	MinPositionSize               float64 `json:"min_position_size"`
	MinRiskRewardRatio            float64 `json:"min_risk_reward_ratio"`
	MinConfidence                 float64 `json:"min_confidence"`
}

// PromptSections 用户可编辑提示词段（VERBATIM 透传，不过滤语言，§9.3）。
type PromptSections struct {
	RoleDefinition   string `json:"role_definition,omitempty"`
	TradingFrequency string `json:"trading_frequency,omitempty"`
	EntryStandards   string `json:"entry_standards,omitempty"`
	DecisionProcess  string `json:"decision_process,omitempty"`
}

// GridStrategyConfig 网格策略配置（独立 builder，§16 无共享段）。
type GridStrategyConfig struct {
	Symbol            string  `json:"symbol,omitempty"`
	GridCount         int     `json:"grid_count,omitempty"`
	LowerPrice        float64 `json:"lower_price,omitempty"`
	UpperPrice        float64 `json:"upper_price,omitempty"`
	Leverage          int     `json:"leverage,omitempty"`
	InvestmentUSD     float64 `json:"investment_usd,omitempty"`
	MaxOrders         int     `json:"max_orders,omitempty"`
}

// StrategyConfig 完整策略配置（§10 StrategyConfig）。
type StrategyConfig struct {
	StrategyType  string            `json:"strategy_type,omitempty"` // ai | grid | dca
	Language      string            `json:"language,omitempty"`      // zh | en
	CoinSource    CoinSourceConfig  `json:"coin_source"`
	Indicators    IndicatorConfig   `json:"indicators"`
	RiskControl   RiskControlConfig `json:"risk_control"`
	CustomPrompt  string            `json:"custom_prompt,omitempty"`
	PromptSections *PromptSections  `json:"prompt_sections,omitempty"`
	GridConfig    *GridStrategyConfig `json:"grid_config,omitempty"`
}

// CreateStrategyRequest 创建策略（config 缺省用系统默认，§11）。
type CreateStrategyRequest struct {
	Name        string          `json:"name" binding:"required,max=64"`
	Description string          `json:"description,omitempty"`
	Lang        string          `json:"lang,omitempty"` // zh | en
	Config      json.RawMessage `json:"config"`
}

// UpdateStrategyRequest 更新策略（读-改-写工作流：PUT 后 GET 验证，§11）。
type UpdateStrategyRequest struct {
	Name        *string         `json:"name,omitempty"`
	Description *string         `json:"description,omitempty"`
	Config      json.RawMessage `json:"config"`
}

// PreviewPromptRequest 提示词预览（§11 POST /strategies/preview-prompt）。
type PreviewPromptRequest struct {
	Config json.RawMessage `json:"config" binding:"required"`
}

// TestRunRequest AI 试跑分析（§11 POST /strategies/test-run）。
type TestRunRequest struct {
	Config json.RawMessage `json:"config" binding:"required"`
}

// StrategyDTO 策略响应（§11 路由契约形状：平铺元数据 + config JSON）。
type StrategyDTO struct {
	ID          string          `json:"id"`
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	IsActive    bool            `json:"is_active"`
	IsDefault   bool            `json:"is_default"`
	IsPublic    bool            `json:"is_public"`
	Config      json.RawMessage `json:"config"` // StrategyConfig JSON
	CreatedAt   time.Time       `json:"created_at"`
	UpdatedAt   time.Time       `json:"updated_at"`
}
