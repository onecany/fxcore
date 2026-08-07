package service

import (
	"net/http"
	"strings"

	"fxcore/internal/api/v1/dto"
	"fxcore/internal/middleware"
	"fxcore/internal/model"
	"fxcore/internal/pkg/crypto"
	"fxcore/internal/store"
)

// ExchangeService 交易所账户：RSA 加密存储、按类型校验必填字段、Key 轮换（API设计.md §11）。
type ExchangeService struct {
	store *store.Store
	km    *crypto.KeyManager
}

// NewExchangeService 构造交易所服务。
func NewExchangeService(s *store.Store, km *crypto.KeyManager) *ExchangeService {
	return &ExchangeService{store: s, km: km}
}

// ValidExchangeTypes 支持的全部 10 家交易所（§10 扩展）。
var ValidExchangeTypes = map[string]bool{
	model.ExchangeBinance: true, model.ExchangeBybit: true, model.ExchangeOKX: true,
	model.ExchangeBitget: true, model.ExchangeGate: true, model.ExchangeKuCoin: true,
	model.ExchangeIndodax: true, model.ExchangeHyperliquid: true,
	model.ExchangeAster: true, model.ExchangeLighter: true,
}

// cexTypes 需要 api_key+secret_key 的中心化交易所。
var cexTypes = map[string]bool{
	model.ExchangeBinance: true, model.ExchangeBybit: true, model.ExchangeOKX: true,
	model.ExchangeBitget: true, model.ExchangeGate: true, model.ExchangeKuCoin: true,
	model.ExchangeIndodax: true,
}

// passphraseTypes 额外需要 passphrase 的交易所（okx/gate/kucoin）。
var passphraseTypes = map[string]bool{
	model.ExchangeOKX: true, model.ExchangeGate: true, model.ExchangeKuCoin: true,
}

// Create 创建交易所：先按类型校验必填字段，再 RSA 加密凭据落库。
func (svc *ExchangeService) Create(in *dto.CreateExchangeRequest) (*model.Exchange, *middleware.APIError) {
	if !ValidExchangeTypes[in.ExchangeType] {
		return nil, middleware.BadRequest("unsupported exchange type", map[string]string{"exchange_type": "must be one of binance,bybit,okx,bitget,gate,kucoin,indodax,hyperliquid,aster,lighter"})
	}
	if strings.TrimSpace(in.AccountName) == "" {
		return nil, middleware.BadRequest("account_name is required", map[string]string{"account_name": "required"})
	}
	if err := svc.validateRequired(in.ExchangeType, in); err != nil {
		return nil, err
	}
	e := &model.Exchange{
		ExchangeType: in.ExchangeType,
		AccountName:  strings.TrimSpace(in.AccountName),
	}
	if in.Enabled != nil {
		e.Enabled = *in.Enabled
	}
	if in.Testnet != nil {
		e.Testnet = *in.Testnet
	}
	if apiErr := svc.applyCredentials(e, in); apiErr != nil {
		return nil, apiErr
	}
	svc.store.CreateExchange(e)
	created, ok := svc.store.GetExchange(e.ID)
	if !ok {
		return nil, middleware.Internal("exchange created but not found")
	}
	return created, nil
}

// Update 部分更新（Partial<CreateExchangeRequest>）：指针字段区分未传/置空；
// 检测到新 Key 时用当前公钥重新加密（Key 轮换，文档 6.2 同语义）。
func (svc *ExchangeService) Update(id string, in *dto.UpdateExchangeRequest) (*model.Exchange, *middleware.APIError) {
	e, ok := svc.store.GetExchange(id)
	if !ok {
		return nil, middleware.NotFound("exchange not found")
	}
	if in.ExchangeType != nil {
		if !ValidExchangeTypes[*in.ExchangeType] {
			return nil, middleware.BadRequest("unsupported exchange type", map[string]string{"exchange_type": "unsupported"})
		}
		e.ExchangeType = *in.ExchangeType
	}
	if in.AccountName != nil {
		e.AccountName = strings.TrimSpace(*in.AccountName)
	}
	if in.Enabled != nil {
		e.Enabled = *in.Enabled
	}
	if in.Testnet != nil {
		e.Testnet = *in.Testnet
	}
	// 合并后的必填校验：类型可能已变更
	full := mergeExchangeFields(e, in)
	if apiErr := svc.validateRequired(e.ExchangeType, full); apiErr != nil {
		return nil, apiErr
	}
	if apiErr := svc.applyCredentialsUpdate(e, in); apiErr != nil {
		return nil, apiErr
	}
	svc.store.UpdateExchange(e)
	return e, nil
}

// List 全部未删除交易所（凭据脱敏在 handler 的 toDTO 完成）。
func (svc *ExchangeService) List() []*model.Exchange {
	return svc.store.ListExchanges()
}

// Get 单个交易所。
func (svc *ExchangeService) Get(id string) (*model.Exchange, *middleware.APIError) {
	e, ok := svc.store.GetExchange(id)
	if !ok {
		return nil, middleware.NotFound("exchange not found")
	}
	return e, nil
}

// Delete 软删除（断开关联交易员语义）。
func (svc *ExchangeService) Delete(id string) *middleware.APIError {
	if !svc.store.DeleteExchange(id) {
		return middleware.NotFound("exchange not found")
	}
	return nil
}

// validateRequired 按交易所类型校验必填凭据字段（§10 字段契约）。
func (svc *ExchangeService) validateRequired(exType string, in *dto.CreateExchangeRequest) *middleware.APIError {
	switch {
	case cexTypes[exType]:
		if strings.TrimSpace(in.APIKey) == "" {
			return middleware.BadRequest("api_key is required for CEX", map[string]string{"api_key": "required"})
		}
		if strings.TrimSpace(in.SecretKey) == "" {
			return middleware.BadRequest("secret_key is required for CEX", map[string]string{"secret_key": "required"})
		}
		if passphraseTypes[exType] && strings.TrimSpace(in.Passphrase) == "" {
			return middleware.BadRequest("passphrase is required for "+exType, map[string]string{"passphrase": "required"})
		}
	case exType == model.ExchangeHyperliquid:
		if strings.TrimSpace(in.HyperliquidWalletAddr) == "" {
			return middleware.BadRequest("hyperliquid_wallet_addr is required", map[string]string{"hyperliquid_wallet_addr": "required"})
		}
	case exType == model.ExchangeAster:
		if strings.TrimSpace(in.AsterUser) == "" || strings.TrimSpace(in.AsterSigner) == "" || strings.TrimSpace(in.AsterPrivateKey) == "" {
			return middleware.BadRequest("aster requires aster_user, aster_signer and aster_private_key", map[string]string{"aster": "all three fields required"})
		}
	case exType == model.ExchangeLighter:
		if strings.TrimSpace(in.LighterWalletAddr) == "" || strings.TrimSpace(in.LighterPrivateKey) == "" ||
			strings.TrimSpace(in.LighterAPIKeyPrivateKey) == "" {
			return middleware.BadRequest("lighter requires wallet addr, private key and api key private key", map[string]string{"lighter": "all three fields required"})
		}
	}
	return nil
}

// applyCredentials 把请求明文凭据加密后写入实体（Create 路径）。
func (svc *ExchangeService) applyCredentials(e *model.Exchange, in *dto.CreateExchangeRequest) *middleware.APIError {
	if in.APIKey != "" {
		enc, err := svc.km.Encrypt([]byte(in.APIKey))
		if err != nil {
			return middleware.NewAPIError(middleware.CodeInternal, http.StatusInternalServerError, "encrypt api_key failed")
		}
		e.APIKeyEnc = enc
		e.APIKeyPrefix = maskExchangeKey(in.APIKey)
	}
	if in.SecretKey != "" {
		enc, err := svc.km.Encrypt([]byte(in.SecretKey))
		if err != nil {
			return middleware.NewAPIError(middleware.CodeInternal, http.StatusInternalServerError, "encrypt secret_key failed")
		}
		e.SecretKeyEnc = enc
	}
	if in.Passphrase != "" {
		enc, err := svc.km.Encrypt([]byte(in.Passphrase))
		if err != nil {
			return middleware.NewAPIError(middleware.CodeInternal, http.StatusInternalServerError, "encrypt passphrase failed")
		}
		e.PassphraseEnc = enc
	}
	if in.AsterPrivateKey != "" {
		enc, err := svc.km.Encrypt([]byte(in.AsterPrivateKey))
		if err != nil {
			return middleware.NewAPIError(middleware.CodeInternal, http.StatusInternalServerError, "encrypt aster_private_key failed")
		}
		e.AsterPrivateKeyEnc = enc
	}
	if in.LighterPrivateKey != "" {
		enc, err := svc.km.Encrypt([]byte(in.LighterPrivateKey))
		if err != nil {
			return middleware.NewAPIError(middleware.CodeInternal, http.StatusInternalServerError, "encrypt lighter_private_key failed")
		}
		e.LighterPrivateKeyEnc = enc
	}
	if in.LighterAPIKeyPrivateKey != "" {
		enc, err := svc.km.Encrypt([]byte(in.LighterAPIKeyPrivateKey))
		if err != nil {
			return middleware.NewAPIError(middleware.CodeInternal, http.StatusInternalServerError, "encrypt lighter_api_key_private_key failed")
		}
		e.LighterAPIKeyPrivateKeyEnc = enc
	}
	e.HyperliquidWalletAddr = in.HyperliquidWalletAddr
	e.AsterUser = in.AsterUser
	e.AsterSigner = in.AsterSigner
	e.LighterWalletAddr = in.LighterWalletAddr
	e.LighterAPIKeyIndex = in.LighterAPIKeyIndex
	return nil
}

// applyCredentialsUpdate 部分更新路径：仅当传入非空明文时才（重）加密。
func (svc *ExchangeService) applyCredentialsUpdate(e *model.Exchange, in *dto.UpdateExchangeRequest) *middleware.APIError {
	encrypt := func(plain string) (string, *middleware.APIError) {
		enc, err := svc.km.Encrypt([]byte(plain))
		if err != nil {
			return "", middleware.NewAPIError(middleware.CodeInternal, http.StatusInternalServerError, "encrypt credential failed")
		}
		return enc, nil
	}
	if in.APIKey != nil && *in.APIKey != "" {
		enc, apiErr := encrypt(*in.APIKey)
		if apiErr != nil {
			return apiErr
		}
		e.APIKeyEnc = enc
		e.APIKeyPrefix = maskExchangeKey(*in.APIKey)
	}
	if in.SecretKey != nil && *in.SecretKey != "" {
		enc, apiErr := encrypt(*in.SecretKey)
		if apiErr != nil {
			return apiErr
		}
		e.SecretKeyEnc = enc
	}
	if in.Passphrase != nil && *in.Passphrase != "" {
		enc, apiErr := encrypt(*in.Passphrase)
		if apiErr != nil {
			return apiErr
		}
		e.PassphraseEnc = enc
	}
	if in.AsterPrivateKey != nil && *in.AsterPrivateKey != "" {
		enc, apiErr := encrypt(*in.AsterPrivateKey)
		if apiErr != nil {
			return apiErr
		}
		e.AsterPrivateKeyEnc = enc
	}
	if in.LighterPrivateKey != nil && *in.LighterPrivateKey != "" {
		enc, apiErr := encrypt(*in.LighterPrivateKey)
		if apiErr != nil {
			return apiErr
		}
		e.LighterPrivateKeyEnc = enc
	}
	if in.LighterAPIKeyPrivateKey != nil && *in.LighterAPIKeyPrivateKey != "" {
		enc, apiErr := encrypt(*in.LighterAPIKeyPrivateKey)
		if apiErr != nil {
			return apiErr
		}
		e.LighterAPIKeyPrivateKeyEnc = enc
	}
	if in.HyperliquidWalletAddr != nil {
		e.HyperliquidWalletAddr = *in.HyperliquidWalletAddr
	}
	if in.AsterUser != nil {
		e.AsterUser = *in.AsterUser
	}
	if in.AsterSigner != nil {
		e.AsterSigner = *in.AsterSigner
	}
	if in.LighterWalletAddr != nil {
		e.LighterWalletAddr = *in.LighterWalletAddr
	}
	if in.LighterAPIKeyIndex != nil {
		e.LighterAPIKeyIndex = *in.LighterAPIKeyIndex
	}
	return nil
}

// maskExchangeKey 脱敏：ab****cd（与 model_svc.maskKey 同风格）。
func maskExchangeKey(key string) string {
	if len(key) <= 8 {
		return "****"
	}
	return key[:2] + "****" + key[len(key)-2:]
}

// mergeExchangeFields 把实体当前值 + 更新请求合并成 Create 形状，供必填校验用。
func mergeExchangeFields(e *model.Exchange, in *dto.UpdateExchangeRequest) *dto.CreateExchangeRequest {
	out := &dto.CreateExchangeRequest{
		ExchangeType: e.ExchangeType,
		AccountName:  e.AccountName,
	}
	// 明文凭据无法从密文还原，仅用于校验"必填"语义：已加密即视为已提供
	if e.APIKeyEnc != "" {
		out.APIKey = "provided"
	}
	if e.SecretKeyEnc != "" {
		out.SecretKey = "provided"
	}
	if e.PassphraseEnc != "" {
		out.Passphrase = "provided"
	}
	if e.AsterPrivateKeyEnc != "" {
		out.AsterPrivateKey = "provided"
	}
	if e.LighterPrivateKeyEnc != "" {
		out.LighterPrivateKey = "provided"
	}
	if e.LighterAPIKeyPrivateKeyEnc != "" {
		out.LighterAPIKeyPrivateKey = "provided"
	}
	out.HyperliquidWalletAddr = e.HyperliquidWalletAddr
	out.AsterUser = e.AsterUser
	out.AsterSigner = e.AsterSigner
	out.LighterWalletAddr = e.LighterWalletAddr
	if in.ExchangeType != nil {
		out.ExchangeType = *in.ExchangeType
	}
	return out
}
