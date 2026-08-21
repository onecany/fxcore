package handler

import (
	"context"
	"time"

	"github.com/gin-gonic/gin"

	"fxcore/internal/api/v1/dto"
	"fxcore/internal/api/v1/service"
	"fxcore/internal/kernel"
	"fxcore/internal/llm"
	"fxcore/internal/middleware"
	"fxcore/internal/model"
	"fxcore/internal/provider"
)

// StrategyHandler 策略 CRUD + 生效/复制 + 默认配置 + 提示词预览/AI 试跑。
// 见 API设计.md §11 strategies 路由。preview-prompt 构建提示词（kernel），
// test-run 真实调 AI（llm.Client + llm.ModelProvider + K 线数据源链组装层注入）。
type StrategyHandler struct {
	svc    *service.StrategyService
	msvc   *service.ModelService // 模型归属校验（TestRun 用）
	ai     *llm.Client
	models llm.ModelProvider // 解密后的模型配置
	klines provider.KlineProvider // 数据源链（AI 试跑拉真实 K 线 + 指标）
}

// NewStrategyHandler 构造策略处理器。
func NewStrategyHandler(svc *service.StrategyService, msvc *service.ModelService, ai *llm.Client, models llm.ModelProvider, klines provider.KlineProvider) *StrategyHandler {
	return &StrategyHandler{svc: svc, msvc: msvc, ai: ai, models: models, klines: klines}
}

// List GET /strategies?page=&size=&fields=
// List 策略列表（分页 + fields）。
// @Summary 策略列表
// @Tags strategies
// @Produce json
// @Param page query int false "页码" default(1)
// @Param size query int false "每页数量" default(20)
// @Param fields query string false "逗号分隔字段白名单"
// @Success 200 {object} dto.ApiResponse[dto.PaginatedData[any]]
// @Failure 401 {object} dto.ErrorResponse "1002 未认证"
// @Router /strategies [get]
func (h *StrategyHandler) List(c *gin.Context) {
	var q dto.ListQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		middleware.WriteError(c, middleware.BadRequest("invalid query", nil))
		return
	}
	page, size := q.Normalized()
	fields := splitFields(q.Fields)
	all := h.svc.List(currentUserID(c))
	total := len(all)
	totalPages := (total + size - 1) / size
	if totalPages == 0 {
		totalPages = 1
	}
	start := (page - 1) * size
	if start > total {
		start = total
	}
	end := start + size
	if end > total {
		end = total
	}
	items := make([]dto.StrategyDTO, 0, end-start)
	for _, st := range all[start:end] {
		items = append(items, strategyToDTO(st))
	}
	if len(fields) > 0 {
		filtered := make([]any, 0, len(items))
		for _, item := range items {
			filtered = append(filtered, dto.FilterFields(item, fields))
		}
		middleware.WriteOK(c, dto.PaginatedData[any]{Items: filtered, Pagination: dto.Paginator{Page: page, PageSize: size, Total: total, TotalPages: totalPages}})
		return
	}
	middleware.WriteOK(c, dto.PaginatedData[dto.StrategyDTO]{Items: items, Pagination: dto.Paginator{Page: page, PageSize: size, Total: total, TotalPages: totalPages}})
}

// Get GET /strategies/{id}
// Get 策略详情（含完整 config）。
// @Summary 策略详情
// @Tags strategies
// @Produce json
// @Param id path string true "策略 ID"
// @Success 200 {object} dto.ApiResponse[dto.StrategyDTO]
// @Failure 404 {object} dto.ErrorResponse "1004 策略不存在"
// @Router /strategies/{id} [get]
func (h *StrategyHandler) Get(c *gin.Context) {
	st, apiErr := h.svc.Get(c.Param("id"), currentUserID(c))
	if apiErr != nil {
		middleware.WriteError(c, apiErr)
		return
	}
	middleware.WriteOK(c, strategyToDTO(st))
}

// GetActive GET /strategies/active
// GetActive 当前生效策略。
// @Summary 当前生效策略
// @Tags strategies
// @Produce json
// @Success 200 {object} dto.ApiResponse[dto.StrategyDTO]
// @Failure 404 {object} dto.ErrorResponse "1004 无生效策略"
// @Router /strategies/active [get]
func (h *StrategyHandler) GetActive(c *gin.Context) {
	st, apiErr := h.svc.GetActive(currentUserID(c))
	if apiErr != nil {
		middleware.WriteError(c, apiErr)
		return
	}
	middleware.WriteOK(c, strategyToDTO(st))
}

// GetDefaultConfig GET /strategies/default-config
// GetDefaultConfig 默认配置参考。
// @Summary 默认策略配置
// @Description 返回系统默认 StrategyConfig（§14.1 风控默认值），创建策略时 config 缺省即用此值
// @Tags strategies
// @Produce json
// @Success 200 {object} dto.ApiResponse[dto.StrategyConfig]
// @Router /strategies/default-config [get]
func (h *StrategyHandler) GetDefaultConfig(c *gin.Context) {
	middleware.WriteOK(c, service.DefaultConfig())
}

// Create POST /strategies
// Create 创建策略（config 缺省用默认）。
// @Summary 创建策略
// @Tags strategies
// @Accept json
// @Produce json
// @Param body body dto.CreateStrategyRequest true "策略配置（config 可选）"
// @Success 200 {object} dto.ApiResponse[dto.StrategyDTO]
// @Failure 400 {object} dto.ErrorResponse "1001 参数错误/非法 config"
// @Router /strategies [post]
func (h *StrategyHandler) Create(c *gin.Context) {
	var req dto.CreateStrategyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.WriteError(c, middleware.BadRequest("invalid request body: "+err.Error(), nil))
		return
	}
	st, apiErr := h.svc.Create(currentUserID(c), &req)
	if apiErr != nil {
		middleware.WriteError(c, apiErr)
		return
	}
	middleware.WriteOK(c, strategyToDTO(st))
}

// Update PUT /strategies/{id}
// Update 更新策略（读-改-写，PUT 后 GET 验证）。
// @Summary 更新策略
// @Tags strategies
// @Accept json
// @Produce json
// @Param id path string true "策略 ID"
// @Param body body dto.UpdateStrategyRequest true "部分更新字段"
// @Success 200 {object} dto.ApiResponse[dto.StrategyDTO]
// @Failure 404 {object} dto.ErrorResponse "1004 策略不存在"
// @Router /strategies/{id} [put]
func (h *StrategyHandler) Update(c *gin.Context) {
	var req dto.UpdateStrategyRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.WriteError(c, middleware.BadRequest("invalid request body: "+err.Error(), nil))
		return
	}
	st, apiErr := h.svc.Update(c.Param("id"), currentUserID(c), &req)
	if apiErr != nil {
		middleware.WriteError(c, apiErr)
		return
	}
	middleware.WriteOK(c, strategyToDTO(st))
}

// Delete DELETE /strategies/{id}
// Delete 删除策略（运行中交易员占用时拒绝）。
// @Summary 删除策略
// @Tags strategies
// @Produce json
// @Param id path string true "策略 ID"
// @Success 200 {object} dto.ApiResponse[any]
// @Failure 409 {object} dto.ErrorResponse "1204 策略被运行中交易员占用"
// @Failure 404 {object} dto.ErrorResponse "1004 策略不存在"
// @Router /strategies/{id} [delete]
func (h *StrategyHandler) Delete(c *gin.Context) {
	if apiErr := h.svc.Delete(c.Param("id"), currentUserID(c)); apiErr != nil {
		middleware.WriteError(c, apiErr)
		return
	}
	middleware.WriteOK[any](c, nil)
}

// Activate POST /strategies/{id}/activate
// Activate 置为生效策略。
// @Summary 激活策略
// @Tags strategies
// @Produce json
// @Param id path string true "策略 ID"
// @Success 200 {object} dto.ApiResponse[any]
// @Failure 404 {object} dto.ErrorResponse "1004 策略不存在"
// @Router /strategies/{id}/activate [post]
func (h *StrategyHandler) Activate(c *gin.Context) {
	activated, apiErr := h.svc.Activate(currentUserID(c), c.Param("id"))
	if apiErr != nil {
		middleware.WriteError(c, apiErr)
		return
	}
	middleware.WriteOK(c, strategyToDTO(activated))
}

// Duplicate POST /strategies/{id}/duplicate
// Duplicate 复制策略（名称加 "(copy)"）。
// @Summary 复制策略
// @Tags strategies
// @Produce json
// @Param id path string true "策略 ID"
// @Success 200 {object} dto.ApiResponse[dto.StrategyDTO]
// @Failure 404 {object} dto.ErrorResponse "1004 策略不存在"
// @Router /strategies/{id}/duplicate [post]
func (h *StrategyHandler) Duplicate(c *gin.Context) {
	st, apiErr := h.svc.Duplicate(currentUserID(c), c.Param("id"))
	if apiErr != nil {
		middleware.WriteError(c, apiErr)
		return
	}
	middleware.WriteOK(c, strategyToDTO(st))
}

// PreviewPrompt POST /strategies/preview-prompt
// PreviewPrompt 提示词预览。
// @Summary 提示词预览
// @Description 基于配置生成系统提示词。当前为基础模板（阶段 3 kernel 就位后替换为完整双路径构建）
// @Tags strategies
// @Accept json
// @Produce json
// @Param body body dto.PreviewPromptRequest true "策略配置"
// @Success 200 {object} dto.ApiResponse[map[string]string]
// @Failure 400 {object} dto.ErrorResponse "1001 非法 config"
// @Router /strategies/preview-prompt [post]
func (h *StrategyHandler) PreviewPrompt(c *gin.Context) {
	var req dto.PreviewPromptRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.WriteError(c, middleware.BadRequest("invalid request body: "+err.Error(), nil))
		return
	}
	cfg := service.DefaultConfig()
	if apiErr := service.MergeConfigInto(&cfg, req.Config); apiErr != nil {
		middleware.WriteError(c, apiErr)
		return
	}
	middleware.WriteOK(c, map[string]string{"prompt": kernel.BuildSystemPrompt(cfg)})
}

// Lint POST /strategies/lint
// Lint 策略配置静态检查：合并默认值后跑 Prompt Lint 规则，返回命中警告。
// @Summary 配置静态检查
// @Description 对请求 config（与 preview-prompt 同构，先经 MergeConfigInto 合并默认值）执行静态规则检查，返回「会被引擎静默忽略或产生误导行为」的配置问题列表
// @Tags strategies
// @Accept json
// @Produce json
// @Param body body dto.LintRequest true "策略配置"
// @Success 200 {object} dto.ApiResponse[dto.LintResponse]
// @Failure 400 {object} dto.ErrorResponse "1001 非法 config"
// @Router /strategies/lint [post]
func (h *StrategyHandler) Lint(c *gin.Context) {
	var req dto.LintRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.WriteError(c, middleware.BadRequest("invalid request body: "+err.Error(), nil))
		return
	}
	cfg := service.DefaultConfig()
	if apiErr := service.MergeConfigInto(&cfg, req.Config); apiErr != nil {
		middleware.WriteError(c, apiErr)
		return
	}
	issues := kernel.Lint(cfg)
	if issues == nil {
		issues = []dto.LintIssue{} // nil slice 序列化为 null → 前端 .length 崩（策略决策同类坑）
	}
	middleware.WriteOK(c, dto.LintResponse{Issues: issues})
}

// TestRun POST /strategies/test-run
// TestRun AI 试跑：用指定模型跑当前配置构建的提示词，返回原始输出 + 解析决策。
// @Summary AI 试跑分析
// @Description 按 config 构建系统提示词（kernel.BuildSystemPrompt），调 model_id 指定的 AI 模型，解析六值决策。AI 调用失败返 1301/1302；解析失败不报错，返回 parsed=false + raw 原始输出
// @Tags strategies
// @Accept json
// @Produce json
// @Param body body dto.TestRunRequest true "策略配置 + 模型 ID"
// @Success 200 {object} dto.ApiResponse[dto.TestRunResponse]
// @Failure 400 {object} dto.ErrorResponse "1001 参数错误"
// @Failure 404 {object} dto.ErrorResponse "1004 模型不存在"
// @Failure 503 {object} dto.ErrorResponse "1301 AI 服务失败"
// @Failure 504 {object} dto.ErrorResponse "1302 AI 服务超时"
// @Router /strategies/test-run [post]
func (h *StrategyHandler) TestRun(c *gin.Context) {
	var req dto.TestRunRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.WriteError(c, middleware.BadRequest("invalid request body: "+err.Error(), nil))
		return
	}
	// 模型归属校验（越权视同不存在）
	m, apiErr := h.msvc.Get(req.ModelID, currentUserID(c))
	if apiErr != nil {
		middleware.WriteError(c, apiErr)
		return
	}
	// 解密 API Key（llm.ModelProvider 从 store 取配置 + RSA 解密）
	// 模型存在但 GetModel=false = RSA 解密失败（key 损坏/私钥不匹配/假数据占位 key）
	lm, ok := h.models.GetModel(m.ID)
	if !ok {
		middleware.WriteError(c, middleware.Internal("decrypt stored api key failed: resolve model config returned false"))
		return
	}
	// 构建系统提示词（与 preview-prompt 同一路径）
	cfg := service.DefaultConfig()
	if apiErr := service.MergeConfigInto(&cfg, req.Config); apiErr != nil {
		middleware.WriteError(c, apiErr)
		return
	}
	prompt := kernel.BuildSystemPrompt(cfg)

	// 拉真实 K 线 + 计算技术指标，作为 user 上下文（2026-08 修复：原实现 user 消息
	// 是硬编码 "Analyze the current market..."，AI 拿不到任何市场数据）
	symbol := ""
	if len(cfg.CoinSource.StaticCoins) > 0 {
		symbol = cfg.CoinSource.StaticCoins[0]
	} else {
		symbol = "BTC-USDT" // 引擎 candidateSymbols 同款兜底
	}
	userContent := "Analyze the current market and output your decision."
	if h.klines != nil {
		tf := cfg.Indicators.Klines.PrimaryTimeframe
		if tf == "" {
			tf = "15m"
		}
		count := cfg.Indicators.Klines.PrimaryCount
		if count <= 0 {
			count = 200
		}
		ks, err := h.klines.Klines(c.Request.Context(), symbol, tf, count)
		if err == nil && len(ks) > 0 {
			userContent = kernel.BuildUserContext(0, nil, kernel.BuildKlineContext(ks, cfg))
		}
	}

	ctx, cancel := context.WithTimeout(c.Request.Context(), 60*time.Second)
	defer cancel()
	res, err := h.ai.Chat(ctx, lm, llm.ChatRequest{
		Messages: []llm.Message{
			{Role: "system", Content: prompt},
			{Role: "user", Content: userContent},
		},
		Temperature: 0.3,
		MaxTokens:   2048,
	})
	if err != nil {
		if err == llm.ErrTimeout {
			middleware.WriteError(c, middleware.AITimeout("AI provider timeout: "+err.Error()))
			return
		}
		middleware.WriteError(c, middleware.AIFailed("AI provider failed: "+err.Error()))
		return
	}
	actions, parseErr := kernel.ParseDecision(res.Content)
	// 解析失败时 actions 为 nil——JSON 序列化会输出 `decisions: null`（Go nil slice 坑），
	// 前端直接读 decisions.length 崩溃。保证契约稳定：失败返回空数组（2026-08 实测 TypeError 实锤）。
	if parseErr != nil && actions == nil {
		actions = []model.DecisionAction{}
	}
	out := dto.TestRunResponse{
		Prompt:    prompt,
		Raw:       res.Content,
		Decisions: actions,
		Parsed:    parseErr == nil,
		LatencyMS: res.LatencyMS,
	}
	if parseErr != nil {
		out.Error = parseErr.Error()
	}
	middleware.WriteOK(c, out)
}

// Public GET /strategies/public
// Public 公开策略市场。骨架：返回空列表（公开市场阶段 3 填充）。
// @Summary 公开策略市场
// @Tags strategies
// @Produce json
// @Success 200 {object} dto.ApiResponse[[]dto.StrategyDTO]
// @Router /strategies/public [get]
func (h *StrategyHandler) Public(c *gin.Context) {
	middleware.WriteOK(c, []dto.StrategyDTO{})
}

// strategyToDTO 实体转 DTO。
func strategyToDTO(st *model.Strategy) dto.StrategyDTO {
	return dto.StrategyDTO{
		ID:          st.ID,
		Name:        st.Name,
		Description: st.Description,
		IsActive:    st.IsActive,
		IsDefault:   st.IsDefault,
		IsPublic:    st.IsPublic,
		Config:      st.Config,
		CreatedAt:   st.CreatedAt,
		UpdatedAt:   st.UpdatedAt,
	}
}

