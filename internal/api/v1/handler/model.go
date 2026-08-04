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
func (h *ModelHandler) Providers(c *gin.Context) {
	middleware.WriteOK(c, providerCatalog)
}

// Create POST /models
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
