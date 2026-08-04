// Package handler HTTP 处理器层（API设计.md 7.1 蓝图的 internal/api/v1/handler/）。
package handler

import (
	"encoding/json"
	"strings"

	"github.com/gin-gonic/gin"

	"fxcore/internal/api/v1/dto"
	"fxcore/internal/api/v1/service"
	"fxcore/internal/middleware"
	"fxcore/internal/model"
)

// ModelHandler AI 模型 CRUD + 连通测试。
type ModelHandler struct {
	svc *service.ModelService
}

// NewModelHandler 构造模型处理器。
func NewModelHandler(svc *service.ModelService) *ModelHandler {
	return &ModelHandler{svc: svc}
}

// providerCatalog 动态下拉数据源（GET /models/providers）。
var providerCatalog = []dto.ProviderOption{
	{Provider: "deepseek", Models: []string{"deepseek-chat", "deepseek-reasoner"}},
	{Provider: "qwen", Models: []string{"qwen-max", "qwen-plus", "qwen-turbo"}},
	{Provider: "claude", Models: []string{"claude-opus-4-1", "claude-sonnet-4-5", "claude-haiku-4-5"}},
	{Provider: "gpt", Models: []string{"gpt-4o", "gpt-4o-mini", "gpt-4-turbo"}},
	{Provider: "gemini", Models: []string{"gemini-2.5-pro", "gemini-2.5-flash"}},
	{Provider: "custom", Models: []string{"custom-endpoint"}},
}

// List GET /models?fields=id,name,status
// List 模型列表（支持 fields 字段过滤）。
// @Summary 模型列表
// @Description 分页? 否——返回全部未删除模型，支持 fields 白名单过滤
// @Tags models
// @Produce json
// @Param fields query string false "逗号分隔的字段白名单，如 id,name"
// @Success 200 {object} dto.ApiResponse[dto.AIModelDTOList]
// @Failure 401 {object} dto.ErrorResponse "1002 未认证"
// @Router /models [get]
func (h *ModelHandler) List(c *gin.Context) {
	var q dto.ListQuery
	if err := c.ShouldBindQuery(&q); err != nil {
		middleware.WriteError(c, middleware.BadRequest("invalid query", nil))
		return
	}
	fields := splitFields(q.Fields)
	models := h.svc.List()
	out := make([]dto.AIModelDTO, 0, len(models))
	for _, m := range models {
		out = append(out, modelToDTO(m))
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

// Providers GET /models/providers
// Providers 支持的 AI 提供商列表。
// @Summary 提供商列表
// @Description 支持的 AI 提供商（deepseek/qwen/gpt/claude/gemini/custom）
// @Tags models
// @Produce json
// @Success 200 {object} dto.ApiResponse[dto.ProviderOptionList]
// @Router /models/providers [get]
func (h *ModelHandler) Providers(c *gin.Context) {
	middleware.WriteOK(c, providerCatalog)
}

// Create POST /models
// Create 创建模型（api_key 后端 RSA-OAEP 加密存储，响应永不回传明文 key）。
// @Summary 创建模型
// @Description API Key 仅以 RSA-OAEP(SHA-256) 密文落库；需要 X-Signature/X-Timestamp/X-Nonce 签名头
// @Tags models
// @Accept json
// @Produce json
// @Param body body dto.CreateModelRequest true "模型配置"
// @Success 200 {object} dto.ApiResponse[dto.AIModelDTO]
// @Failure 400 {object} dto.ErrorResponse "1001 参数错误 / 签名缺失或重放"
// @Failure 401 {object} dto.ErrorResponse "1002 未认证"
// @Router /models [post]
func (h *ModelHandler) Create(c *gin.Context) {
	var req dto.CreateModelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.WriteError(c, middleware.BadRequest("invalid request body: "+err.Error(), nil))
		return
	}
	m, apiErr := h.svc.Create(&req)
	if apiErr != nil {
		middleware.WriteError(c, apiErr)
		return
	}
	middleware.WriteOK(c, modelToDTO(m))
}

// Update PUT /models/{id}（含 Key 轮换）
// Update 更新模型（PUT 全量语义；检测到新 api_key 时轮换加密并记录 rotated_at）。
// @Summary 更新模型
// @Description Key 轮换：传新 api_key 时用当前公钥重新加密；custom 需要 base_url
// @Tags models
// @Accept json
// @Produce json
// @Param id path string true "模型 ID"
// @Param body body dto.CreateModelRequest true "模型配置"
// @Success 200 {object} dto.ApiResponse[dto.AIModelDTO]
// @Failure 400 {object} dto.ErrorResponse "1001 参数错误"
// @Failure 404 {object} dto.ErrorResponse "1004 模型不存在"
// @Router /models/{id} [put]
func (h *ModelHandler) Update(c *gin.Context) {
	id := c.Param("id")
	var req dto.CreateModelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.WriteError(c, middleware.BadRequest("invalid request body: "+err.Error(), nil))
		return
	}
	m, apiErr := h.svc.Update(id, &req)
	if apiErr != nil {
		middleware.WriteError(c, apiErr)
		return
	}
	middleware.WriteOK(c, modelToDTO(m))
}

// Delete DELETE /models/{id}（软删除）
// Delete 删除模型（软删除：保留记录但列表不再返回）。
// @Summary 删除模型
// @Description 软删除
// @Tags models
// @Produce json
// @Param id path string true "模型 ID"
// @Success 200 {object} dto.ApiResponse[any]
// @Failure 404 {object} dto.ErrorResponse "1004 模型不存在"
// @Router /models/{id} [delete]
func (h *ModelHandler) Delete(c *gin.Context) {
	if apiErr := h.svc.Delete(c.Param("id")); apiErr != nil {
		middleware.WriteError(c, apiErr)
		return
	}
	middleware.WriteOK[any](c, nil)
}

// Test POST /models/{id}/test 连通测试。
// 请求体可选（TestModelRequest 为空时用已存 Key）。
// 成功 -> 200 + TestResult；失败 -> 1301/1302 业务错误信封（前端据此显示重试/自动重试）。
// Test 连通测试。
// @Summary 模型连通测试
// @Description 请求体可选（key 省略时用已存 Key）；成功返回延迟，失败返回 1301/1302
// @Tags models
// @Accept json
// @Produce json
// @Param id path string true "模型 ID"
// @Param body body dto.TestModelRequest false "临时 api_key（可选）"
// @Success 200 {object} dto.ApiResponse[dto.TestResult]
// @Failure 404 {object} dto.ErrorResponse "1004 模型不存在"
// @Failure 503 {object} dto.ErrorResponse "1301 AI 服务失败"
// @Failure 504 {object} dto.ErrorResponse "1302 AI 服务超时"
// @Router /models/{id}/test [post]
func (h *ModelHandler) Test(c *gin.Context) {
	m, apiErr := h.svc.Get(c.Param("id"))
	if apiErr != nil {
		middleware.WriteError(c, apiErr)
		return
	}
	var req dto.TestModelRequest
	_ = c.ShouldBindJSON(&req) // 可选 body

	latency, testErr := h.svc.Test(m, req.APIKey)
	if testErr != nil {
		middleware.WriteError(c, testErr) // 1301 AI 失败 / 1302 AI 超时
		return
	}
	middleware.WriteOK(c, dto.TestResult{Success: true, Latency: latency})
}

// modelToDTO model -> DTO（api_key 永不出现）。
func modelToDTO(m *model.AIModel) dto.AIModelDTO {
	return dto.AIModelDTO{
		ID:           m.ID,
		Name:         m.Name,
		Provider:     m.Provider,
		ModelName:    m.ModelName,
		APIKeyPrefix: m.APIKeyPrefix,
		Status:       m.Status,
		Config:       configRaw(m.Config),
		CreatedAt:    m.CreatedAt,
		LastTestAt:   m.LastTestAt,
		RotatedAt:    m.RotatedAt,
	}
}

// configRaw 空配置转 nil RawMessage。
// 注意：json.RawMessage("")（非 nil 空切片）会让 json.Marshal 返回
// "unexpected end of JSON input" 并输出 0 字节——必须归一为 nil（序列化为 null）。
func configRaw(s string) json.RawMessage {
	if s == "" || s == "null" {
		return nil
	}
	return json.RawMessage(s)
}

func splitFields(f string) []string {
	if f == "" {
		return nil
	}
	parts := strings.Split(f, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
