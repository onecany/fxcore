package service

import (
	"encoding/json"
	"strings"

	"fxcore/internal/api/v1/dto"
	"fxcore/internal/middleware"
	"fxcore/internal/model"
	"fxcore/internal/store"
)

// StrategyService 策略 CRUD + 生效管理 + 默认配置（API设计.md §11）。
type StrategyService struct {
	store *store.Store
}

// NewStrategyService 构造策略服务。
func NewStrategyService(s *store.Store) *StrategyService {
	return &StrategyService{store: s}
}

// DefaultConfig 系统默认策略配置（§14.1 风控强制 + §16 默认指标）。
// 风控默认值来自契约：max_positions=3、BTC/ETH 仓值 ≤5×equity、alt ≤1×equity、
// 最小仓位 12 USDT、保证金 ≤30%、BTC/ETH 杠杆 5 / alt 杠杆 5。
func DefaultConfig() dto.StrategyConfig {
	return dto.StrategyConfig{
		StrategyType:  "ai",
		Language:      "zh",
		PromptVariant: "balanced", // §9.5 默认模式
		CoinSource: dto.CoinSourceConfig{
			SourceType: dto.CoinSourceStatic,
			UseAI500:   false,
		},
		Indicators: dto.IndicatorConfig{
			Klines: dto.KlineConfig{
				PrimaryTimeframe:     "15m",
				PrimaryCount:         200,
				EnableMultiTimeframe: true,
				SelectedTimeframes:   []string{"1m", "5m", "15m", "1h", "4h", "1d"},
			},
			EnableRawKlines: true,
			EnableEMA:       true,
			EMAPeriods:      []int{7, 25, 99},
			EnableMACD:      true,
			EnableRSI:       true,
			RSIPeriods:      []int{14},
			EnableATR:       true,
			ATRPeriods:      []int{14},
			EnableBoll:      true,
			BollPeriods:     []int{20},
			EnableVolume:    true,
			EnableOI:        false,
			EnableFundingRate: false,
		},
		RiskControl: dto.RiskControlConfig{
			MaxPositions:                 3,
			BTCEthMaxLeverage:            5,
			AltcoinMaxLeverage:           5,
			BTCEthMaxPositionValueRatio:  5,
			AltcoinMaxPositionValueRatio: 1,
			MaxMarginUsage:               0.3,
			MinPositionSize:              12,
			MinRiskRewardRatio:           1.5,
			MinConfidence:                0.6,
		},
	}
}

// Create 创建策略：config 缺省用系统默认，提供则按顶层 section 合并默认（§11）。
func (svc *StrategyService) Create(userID string, in *dto.CreateStrategyRequest) (*model.Strategy, *middleware.APIError) {
	name := strings.TrimSpace(in.Name)
	if name == "" {
		return nil, middleware.BadRequest("name is required", map[string]string{"name": "required"})
	}
	cfg := DefaultConfig()
	if in.Lang == "en" {
		cfg.Language = "en"
	}
	if len(in.Config) > 0 {
		if apiErr := MergeConfigInto(&cfg, in.Config); apiErr != nil {
			return nil, apiErr
		}
	}
	raw, err := json.Marshal(cfg)
	if err != nil {
		return nil, middleware.Internal("marshal strategy config failed")
	}
	st := &model.Strategy{
		UserID: userID,
		Name:        name,
		Description: in.Description,
		IsActive:    false,
		IsPublic:    false,
		Config:      raw,
	}
	svc.store.CreateStrategy(st)
	created, ok := svc.store.GetStrategy(st.ID)
	if !ok {
		return nil, middleware.Internal("strategy created but not found")
	}
	return created, nil
}

// Update 读-改-写工作流（§11）：现有 config 与请求字段按顶层 section 合并，
// 未提及字段保留，不清零（与 nofx 部分合并语义一致）。
func (svc *StrategyService) Update(id, userID string, in *dto.UpdateStrategyRequest) (*model.Strategy, *middleware.APIError) {
	st, ok := svc.store.GetStrategy(id)
	if !ok {
		return nil, middleware.NotFound("strategy not found")
	}
	if userID != "" && st.UserID != "" && st.UserID != userID {
		return nil, middleware.NotFound("strategy not found")
	}
	cfg := DefaultConfig()
	if len(st.Config) > 0 {
		_ = json.Unmarshal(st.Config, &cfg) // 存量 config 解析失败则回退默认
	}
	if len(in.Config) > 0 {
		if apiErr := MergeConfigInto(&cfg, in.Config); apiErr != nil {
			return nil, apiErr
		}
	}
	if in.Name != nil && strings.TrimSpace(*in.Name) != "" {
		st.Name = strings.TrimSpace(*in.Name)
	}
	if in.Description != nil {
		st.Description = *in.Description
	}
	raw, err := json.Marshal(cfg)
	if err != nil {
		return nil, middleware.Internal("marshal strategy config failed")
	}
	st.Config = raw
	svc.store.UpdateStrategy(st)
	return st, nil
}

// List 全部策略。
func (svc *StrategyService) List(userID string) []*model.Strategy {
	return svc.store.ListStrategies(userID)
}

// Get 单个策略（含完整 config）。
func (svc *StrategyService) Get(id, userID string) (*model.Strategy, *middleware.APIError) {
	st, ok := svc.store.GetStrategy(id)
	if !ok {
		return nil, middleware.NotFound("strategy not found")
	}
	if userID != "" && st.UserID != "" && st.UserID != userID {
		return nil, middleware.NotFound("strategy not found")
	}
	return st, nil
}

// Delete 删除策略；运行中交易员（running/paused）引用时拒绝（§11）。
func (svc *StrategyService) Delete(id, userID string) *middleware.APIError {
	st, ok := svc.store.GetStrategy(id)
	if !ok || (userID != "" && st.UserID != userID) {
		return middleware.NotFound("strategy not found")
	}
	for _, t := range svc.store.ListTraders("") { // 跨用户检查：任何用户的运行中交易员引用都拒绝
		if t.StrategyID == id && (t.Status == model.StatusRunning || t.Status == model.StatusPaused) {
			return middleware.NewAPIError(middleware.CodeTraderRunning, 409, "strategy is in use by a running trader")
		}
	}
	if !svc.store.DeleteStrategy(id) {
		return middleware.NotFound("strategy not found")
	}
	return nil
}

// Activate 置为生效策略（is_active 全局唯一；归属校验：他人策略视同不存在）。
func (svc *StrategyService) Activate(userID, id string) (*model.Strategy, *middleware.APIError) {
	st, ok := svc.store.GetStrategy(id)
	if !ok || (userID != "" && st.UserID != userID) {
		return nil, middleware.NotFound("strategy not found")
	}
	activated, ok := svc.store.ActivateStrategy(userID, id)
	if !ok {
		return nil, middleware.NotFound("strategy not found")
	}
	return activated, nil
}

// Duplicate 复制策略，名称加 "(copy)"（§11；归属校验：他人策略视同不存在，副本归属当前用户）。
func (svc *StrategyService) Duplicate(userID, id string) (*model.Strategy, *middleware.APIError) {
	src, ok := svc.store.GetStrategy(id)
	if !ok || (userID != "" && src.UserID != "" && src.UserID != userID) {
		return nil, middleware.NotFound("strategy not found")
	}
	cp := &model.Strategy{
		UserID:      userID,
		Name:        src.Name + " (copy)",
		Description: src.Description,
		IsActive:    false,
		IsPublic:    false,
		Config:      append(json.RawMessage(nil), src.Config...),
	}
	svc.store.CreateStrategy(cp)
	created, ok := svc.store.GetStrategy(cp.ID)
	if !ok {
		return nil, middleware.Internal("strategy duplicated but not found")
	}
	return created, nil
}

// GetActive 当前生效策略。
func (svc *StrategyService) GetActive(userID string) (*model.Strategy, *middleware.APIError) {
	st, ok := svc.store.GetActiveStrategy(userID)
	if !ok {
		return nil, middleware.NotFound("no active strategy")
	}
	return st, nil
}

// MergeConfigInto 把请求 config（StrategyConfig JSON）按顶层 section 合并进目标。
// 规则：请求中 source_type 非空 → 整体替换 coin_source；klines.primary_timeframe
// 非空 → 整体替换 indicators；max_positions>0 → 整体替换 risk_control；
// custom_prompt/prompt_sections/grid_config 显式覆盖（允许清空）。
// 导出供 handler 的 preview-prompt 复用。
func MergeConfigInto(dst *dto.StrategyConfig, raw json.RawMessage) *middleware.APIError {
	var req struct {
		StrategyType   *string                `json:"strategy_type"`
		Language       *string                `json:"language"`
		PromptVariant  *string                `json:"prompt_variant"`
		CoinSource     *dto.CoinSourceConfig  `json:"coin_source"`
		Indicators     *dto.IndicatorConfig   `json:"indicators"`
		RiskControl    *dto.RiskControlConfig `json:"risk_control"`
		CustomPrompt   *string                `json:"custom_prompt"`
		PromptSections *dto.PromptSections    `json:"prompt_sections"`
		GridConfig     json.RawMessage        `json:"grid_config"`
	}
	if err := json.Unmarshal(raw, &req); err != nil {
		return middleware.BadRequest("invalid config: "+err.Error(), nil)
	}
	if req.StrategyType != nil {
		dst.StrategyType = *req.StrategyType
	}
	if req.Language != nil {
		dst.Language = *req.Language
	}
	if req.PromptVariant != nil {
		dst.PromptVariant = *req.PromptVariant
	}
	if req.CoinSource != nil && req.CoinSource.SourceType != "" {
		dst.CoinSource = *req.CoinSource
	}
	if req.Indicators != nil && req.Indicators.Klines.PrimaryTimeframe != "" {
		dst.Indicators = *req.Indicators
	}
	if req.RiskControl != nil && req.RiskControl.MaxPositions > 0 {
		dst.RiskControl = *req.RiskControl
	}
	if req.CustomPrompt != nil {
		dst.CustomPrompt = *req.CustomPrompt
	}
	if req.PromptSections != nil {
		dst.PromptSections = req.PromptSections
	}
	if len(req.GridConfig) > 0 {
		dst.GridConfig = &dto.GridStrategyConfig{}
		if err := json.Unmarshal(req.GridConfig, dst.GridConfig); err != nil {
			return middleware.BadRequest("invalid grid_config: "+err.Error(), nil)
		}
	}
	return nil
}
