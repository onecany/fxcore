package handler

import (
	"github.com/gin-gonic/gin"

	"fxcore/internal/api/v1/dto"
	"fxcore/internal/api/v1/service"
	"fxcore/internal/middleware"
	"fxcore/internal/model"
)

// ExchangeHandler 交易所账户 CRUD（凭据脱敏返回）。见 API设计.md §11 exchanges 路由。
type ExchangeHandler struct {
	svc *service.ExchangeService
}

// NewExchangeHandler 构造交易所处理器。
func NewExchangeHandler(svc *service.ExchangeService) *ExchangeHandler {
	return &ExchangeHandler{svc: svc}
}

// List GET /exchanges?fields=
// List 交易所列表（凭据脱敏，支持 fields 过滤）。
// @Summary 交易所列表
// @Description 返回全部未删除交易所账户，凭据列脱敏（api_key_prefix）
// @Tags exchanges
// @Produce json
// @Param fields query string false "逗号分隔字段白名单"
// @Success 200 {object} dto.ApiResponse[[]dto.ExchangeDTO]
// @Failure 401 {object} dto.ErrorResponse "1002 未认证"
// @Router /exchanges [get]
func (h *ExchangeHandler) List(c *gin.Context) {
	var q dto.ListQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		middleware.WriteError(c, middleware.BadRequest("invalid query", nil))
		return
	}
	fields := splitFields(q.Fields)
	list := h.svc.List(currentUserID(c))
	out := make([]dto.ExchangeDTO, 0, len(list))
	for _, e := range list {
		out = append(out, exchangeToDTO(e))
	}
	if len(fields) > 0 {
		filtered := make([]any, 0, len(out))
		for _, item := range out {
			filtered = append(filtered, dto.FilterFields(item, fields))
		}
		middleware.WriteOK(c, filtered)
		return
	}
	middleware.WriteOK(c, out)
}

// Create POST /exchanges
// Create 创建交易所（写操作需签名头）。
// @Summary 创建交易所
// @Description 按类型校验必填凭据字段，RSA 加密存储，返回脱敏实体
// @Tags exchanges
// @Accept json
// @Produce json
// @Param body body dto.CreateExchangeRequest true "交易所配置（凭据原文）"
// @Success 200 {object} dto.ApiResponse[dto.ExchangeDTO]
// @Failure 400 {object} dto.ErrorResponse "1001 参数错误/必填凭据缺失"
// @Failure 401 {object} dto.ErrorResponse "1002 未认证"
// @Router /exchanges [post]
func (h *ExchangeHandler) Create(c *gin.Context) {
	var req dto.CreateExchangeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.WriteError(c, middleware.BadRequest("invalid request body: "+err.Error(), nil))
		return
	}
	e, apiErr := h.svc.Create(currentUserID(c), &req)
	if apiErr != nil {
		middleware.WriteError(c, apiErr)
		return
	}
	middleware.WriteOK(c, exchangeToDTO(e))
}

// Update PUT /exchanges/{id}
// Update 更新交易所（Key 轮换 / 启停）。
// @Summary 更新交易所
// @Description 部分更新语义；传入新 api_key 时触发重新加密（轮换）
// @Tags exchanges
// @Accept json
// @Produce json
// @Param id path string true "交易所 ID"
// @Param body body dto.UpdateExchangeRequest true "部分更新字段"
// @Success 200 {object} dto.ApiResponse[dto.ExchangeDTO]
// @Failure 400 {object} dto.ErrorResponse "1001 参数错误"
// @Failure 404 {object} dto.ErrorResponse "1004 交易所不存在"
// @Router /exchanges/{id} [put]
func (h *ExchangeHandler) Update(c *gin.Context) {
	var req dto.UpdateExchangeRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.WriteError(c, middleware.BadRequest("invalid request body: "+err.Error(), nil))
		return
	}
	e, apiErr := h.svc.Update(c.Param("id"), currentUserID(c), &req)
	if apiErr != nil {
		middleware.WriteError(c, apiErr)
		return
	}
	middleware.WriteOK(c, exchangeToDTO(e))
}

// Delete DELETE /exchanges/{id}
// Delete 删除交易所（软删，断开关联交易员）。
// @Summary 删除交易所
// @Tags exchanges
// @Produce json
// @Param id path string true "交易所 ID"
// @Success 200 {object} dto.ApiResponse[any]
// @Failure 404 {object} dto.ErrorResponse "1004 交易所不存在"
// @Router /exchanges/{id} [delete]
func (h *ExchangeHandler) Delete(c *gin.Context) {
	if apiErr := h.svc.Delete(c.Param("id"), currentUserID(c)); apiErr != nil {
		middleware.WriteError(c, apiErr)
		return
	}
	middleware.WriteOK[any](c, nil)
}

// exchangeToDTO 实体转脱敏 DTO：凭据列全部丢弃，仅保留前缀掩码。
func exchangeToDTO(e *model.Exchange) dto.ExchangeDTO {
	return dto.ExchangeDTO{
		ID:                    e.ID,
		ExchangeType:          e.ExchangeType,
		AccountName:           e.AccountName,
		Enabled:               e.Enabled,
		Testnet:               e.Testnet,
		APIKeyPrefix:          e.APIKeyPrefix,
		HyperliquidWalletAddr: e.HyperliquidWalletAddr,
		AsterUser:             e.AsterUser,
		AsterSigner:           e.AsterSigner,
		LighterWalletAddr:     e.LighterWalletAddr,
		LighterAPIKeyIndex:    e.LighterAPIKeyIndex,
		CreatedAt:             e.CreatedAt,
		UpdatedAt:             e.UpdatedAt,
	}
}

// ListBalances GET /exchanges/balances
// ListBalances 各交易所账户余额（调各所账户 API 实时查询；失败降级 balance=null + error）。
// @Summary 交易所账户余额
// @Tags exchanges
// @Produce json
// @Success 200 {object} dto.ApiResponse[[]dto.ExchangeBalanceDTO]
// @Failure 401 {object} dto.ErrorResponse "1002 未认证"
// @Router /exchanges/balances [get]
func (h *ExchangeHandler) ListBalances(c *gin.Context) {
	middleware.WriteOK(c, h.svc.Balances(currentUserID(c)))
}
