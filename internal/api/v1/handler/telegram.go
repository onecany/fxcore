package handler

import (
	"github.com/gin-gonic/gin"

	"fxcore/internal/api/v1/dto"
	"fxcore/internal/api/v1/service"
	"fxcore/internal/middleware"
	"fxcore/internal/model"
)

// TelegramHandler Telegram 配置 CRUD（API设计.md §11 telegram 路由）。
// bot 客户端接线与 /start 绑定流程属引擎层（阶段 3）。
type TelegramHandler struct {
	svc *service.TelegramService
}

// NewTelegramHandler 构造 Telegram 处理器。
func NewTelegramHandler(svc *service.TelegramService) *TelegramHandler {
	return &TelegramHandler{svc: svc}
}

// Get GET /telegram
// Get 当前 Telegram 配置（token 脱敏）。
// @Summary Telegram 配置
// @Tags telegram
// @Produce json
// @Success 200 {object} dto.ApiResponse[dto.TelegramConfigDTO]
// @Failure 404 {object} dto.ErrorResponse "1004 未配置"
// @Router /telegram [get]
func (h *TelegramHandler) Get(c *gin.Context) {
	cfg, ok := h.svc.Get()
	if !ok {
		middleware.WriteError(c, middleware.NotFound("telegram not configured"))
		return
	}
	prefix := ""
	if cfg.BotTokenEnc != "" {
		if plain, err := decryptToken(cfg.BotTokenEnc, h); err == nil {
			prefix = service.BotTokenPrefix(plain)
		} else {
			prefix = "****"
		}
	}
	middleware.WriteOK(c, telegramToDTO(cfg, prefix))
}

// Save POST /telegram
// Save 保存配置并绑定（写操作需签名头）。
// @Summary 保存 Telegram 配置
// @Description bot_token RSA 加密存储；保存后由 bot 客户端 /start 建立绑定
// @Tags telegram
// @Accept json
// @Produce json
// @Param body body dto.SaveTelegramRequest true "bot_token + model_id"
// @Success 200 {object} dto.ApiResponse[any]
// @Failure 400 {object} dto.ErrorResponse "1001 参数错误"
// @Failure 404 {object} dto.ErrorResponse "1004 模型不存在"
// @Router /telegram [post]
func (h *TelegramHandler) Save(c *gin.Context) {
	var req dto.SaveTelegramRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.WriteError(c, middleware.BadRequest("invalid request body: "+err.Error(), nil))
		return
	}
	if apiErr := h.svc.Save(&req); apiErr != nil {
		middleware.WriteError(c, apiErr)
		return
	}
	middleware.WriteOK[any](c, nil)
}

// SetModel POST /telegram/model
// SetModel 仅更换 AI 模型（保留 token 与绑定）。
// @Summary 更换 Telegram 模型
// @Tags telegram
// @Accept json
// @Produce json
// @Param body body dto.SetTelegramModelRequest true "新 model_id"
// @Success 200 {object} dto.ApiResponse[any]
// @Failure 404 {object} dto.ErrorResponse "1004 未配置/模型不存在"
// @Router /telegram/model [post]
func (h *TelegramHandler) SetModel(c *gin.Context) {
	var req dto.SetTelegramModelRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.WriteError(c, middleware.BadRequest("invalid request body: "+err.Error(), nil))
		return
	}
	if apiErr := h.svc.SetModel(&req); apiErr != nil {
		middleware.WriteError(c, apiErr)
		return
	}
	middleware.WriteOK[any](c, nil)
}

// DeleteBinding DELETE /telegram/binding
// DeleteBinding 解绑：清空配置。
// @Summary 解绑 Telegram
// @Tags telegram
// @Produce json
// @Success 200 {object} dto.ApiResponse[any]
// @Router /telegram/binding [delete]
func (h *TelegramHandler) DeleteBinding(c *gin.Context) {
	h.svc.Delete()
	middleware.WriteOK[any](c, nil)
}

// decryptToken 解密 bot token（取明文做前缀脱敏）。
func decryptToken(enc string, h *TelegramHandler) (string, error) {
	// 通过 service 暴露的 km 解密；此处直接调用 TokenPrefix 内部逻辑
	return h.svc.DecryptToken(enc)
}

// telegramToDTO 实体转 DTO。
func telegramToDTO(cfg *model.TelegramConfig, prefix string) dto.TelegramConfigDTO {
	out := dto.TelegramConfigDTO{
		BotTokenPrefix: prefix,
		ChatID:         cfg.ChatID,
		Username:       cfg.Username,
		ModelID:        cfg.ModelID,
		Language:       cfg.Language,
	}
	if cfg.BoundAt != nil {
		out.BoundAt = cfg.BoundAt.Unix()
	}
	return out
}
