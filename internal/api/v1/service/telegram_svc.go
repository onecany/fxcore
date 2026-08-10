package service

import (
	"fxcore/internal/api/v1/dto"
	"fxcore/internal/middleware"
	"fxcore/internal/model"
	"fxcore/internal/pkg/crypto"
	"fxcore/internal/store"
)

// TelegramService Telegram 绑定配置：token 加密存储、模型切换、解绑（API设计.md §11）。
// bot 客户端与 /start 绑定流程属引擎层（阶段 3），本服务负责配置 CRUD 契约。
type TelegramService struct {
	store *store.Store
	km    *crypto.KeyManager
}

// NewTelegramService 构造 Telegram 服务。
func NewTelegramService(s *store.Store, km *crypto.KeyManager) *TelegramService {
	return &TelegramService{store: s, km: km}
}

// Save 保存配置：bot_token RSA 加密 + 校验 model 存在；已绑定 chat 信息保留（单例 upsert）。
func (svc *TelegramService) Save(userID string, in *dto.SaveTelegramRequest) *middleware.APIError {
	m, ok := svc.store.GetModel(in.ModelID)
	if !ok || (userID != "" && m.UserID != "" && m.UserID != userID) {
		return middleware.NotFound("model not found")
	}
	enc, err := svc.km.Encrypt([]byte(in.BotToken))
	if err != nil {
		return middleware.Internal("encrypt bot token failed")
	}
	cfg := &model.TelegramConfig{
		BotTokenEnc: enc,
		ModelID:     in.ModelID,
		ChatID:      in.ChatID,
	}
	svc.store.UpsertTelegramConfig(userID, cfg)
	return nil
}

// Get 当前配置（token 脱敏在 handler 层）。
func (svc *TelegramService) Get(userID string) (*model.TelegramConfig, bool) {
	return svc.store.GetTelegramConfig(userID)
}

// SetModel 仅换模型：保留 token 与绑定信息。
func (svc *TelegramService) SetModel(userID string, in *dto.SetTelegramModelRequest) *middleware.APIError {
	cur, ok := svc.store.GetTelegramConfig(userID)
	if !ok {
		return middleware.NotFound("telegram not configured")
	}
	m, ok := svc.store.GetModel(in.ModelID)
	if !ok || (userID != "" && m.UserID != "" && m.UserID != userID) {
		return middleware.NotFound("model not found")
	}
	cfg := &model.TelegramConfig{
		BotTokenEnc: cur.BotTokenEnc,
		ModelID:     in.ModelID,
		ChatID:      cur.ChatID,
		Username:    cur.Username,
		BoundAt:     cur.BoundAt,
		Language:    cur.Language,
	}
	svc.store.UpsertTelegramConfig(userID, cfg)
	return nil
}

// Delete 解绑：清空配置（含加密 token）。
func (svc *TelegramService) Delete(userID string) {
	svc.store.DeleteTelegramConfig(userID)
}

// DecryptToken 解密加密 token（handler 层做前缀脱敏用）。
func (svc *TelegramService) DecryptToken(enc string) (string, error) {
	plain, err := svc.km.Decrypt(enc)
	if err != nil {
		return "", err
	}
	return string(plain), nil
}

// BotTokenPrefix 脱敏 token 前缀：12345****。
func BotTokenPrefix(token string) string {
	if len(token) <= 6 {
		return "****"
	}
	return token[:5] + "****"
}
