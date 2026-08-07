package handler

import (
	"encoding/base64"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"fxcore/internal/api/v1/dto"
	"fxcore/internal/middleware"
	"fxcore/internal/pkg/crypto"
)

// CryptoHandler 加密工具端点（API设计.md §11 crypto 路由）。
// 传输加密：前端 RSA-OAEP 包 AES key（wrapped_key）+ AES-GCM 加密业务数据，
// 服务端私钥解包后解密。aad=ts+path 防密文跨端点重放。
type CryptoHandler struct {
	km *crypto.KeyManager
}

// NewCryptoHandler 构造加密处理器。
func NewCryptoHandler(km *crypto.KeyManager) *CryptoHandler {
	return &CryptoHandler{km: km}
}

// maxTSDrift 时间戳容差（与签名中间件一致 ±5 分钟）。
const maxTSDrift = 5 * 60 * 1000 // ms

// GetConfig GET /crypto/config
// GetConfig 传输加密配置。
// @Summary 传输加密配置
// @Tags crypto
// @Produce json
// @Success 200 {object} dto.ApiResponse[dto.CryptoConfigDTO]
// @Router /crypto/config [get]
func (h *CryptoHandler) GetConfig(c *gin.Context) {
	middleware.WriteOK(c, dto.CryptoConfigDTO{TransportEncryption: "aes-256-gcm"})
}

// GetPublicKey GET /crypto/public-key
// GetPublicKey RSA-2048 公钥（PEM）。
// @Summary 获取 RSA 公钥
// @Description 前端用公钥包裹随机 AES key（RSA-OAEP），业务数据用 AES-GCM 加密
// @Tags crypto
// @Produce json
// @Success 200 {object} dto.ApiResponse[dto.PublicKeyDTO]
// @Router /crypto/public-key [get]
func (h *CryptoHandler) GetPublicKey(c *gin.Context) {
	middleware.WriteOK(c, dto.PublicKeyDTO{
		PublicKey: h.km.PublicKeyPEM(),
		Algorithm: "RSA-OAEP-256",
	})
}

// Decrypt POST /crypto/decrypt
// Decrypt 解密前端密文（认证 + TS 校验）。
// @Summary 服务端解密
// @Description 校验 ts ±5min 与 aad（ts+path）匹配后，RSA 解包 AES key、AES-GCM 解密。需签名头
// @Tags crypto
// @Accept json
// @Produce json
// @Param body body dto.DecryptRequest true "混合加密载荷"
// @Success 200 {object} dto.ApiResponse[dto.DecryptResponse]
// @Failure 400 {object} dto.ErrorResponse "1001 ts 过期 / aad 不匹配 / 解密失败"
// @Router /crypto/decrypt [post]
func (h *CryptoHandler) Decrypt(c *gin.Context) {
	var req dto.DecryptRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		middleware.WriteError(c, middleware.BadRequest("invalid request body: "+err.Error(), nil))
		return
	}
	ts, err := strconv.ParseInt(req.TS, 10, 64)
	if err != nil {
		middleware.WriteError(c, middleware.BadRequest("invalid ts", map[string]string{"ts": "must be unix millis"}))
		return
	}
	if delta := time.Now().UnixMilli() - ts; delta > maxTSDrift || delta < -maxTSDrift {
		middleware.WriteError(c, middleware.BadRequest("ts out of ±5min window", map[string]string{"ts": "expired"}))
		return
	}
	// aad 必须等于 ts+path：防密文在另一个端点重放
	expectedAAD := req.TS + c.Request.URL.Path
	if req.AAD != expectedAAD {
		middleware.WriteError(c, middleware.BadRequest("aad mismatch", map[string]string{"aad": "must be ts+path"}))
		return
	}
	plain, apiErr := h.decryptPayload(req.WrappedKey, req.IV, req.Ciphertext, req.AAD)
	if apiErr != nil {
		middleware.WriteError(c, apiErr)
		return
	}
	middleware.WriteOK(c, dto.DecryptResponse{Plaintext: plain})
}

// decryptPayload RSA-OAEP 解包 AES key → AES-GCM 解密。
func (h *CryptoHandler) decryptPayload(wrappedKeyB64, ivB64, cipherB64, aad string) (string, *middleware.APIError) {
	aesKey, err := h.km.Decrypt(wrappedKeyB64)
	if err != nil {
		return "", middleware.BadRequest("unwrap aes key failed", nil)
	}
	iv, err := base64.StdEncoding.DecodeString(ivB64)
	if err != nil {
		return "", middleware.BadRequest("invalid iv", nil)
	}
	ct, err := base64.StdEncoding.DecodeString(cipherB64)
	if err != nil {
		return "", middleware.BadRequest("invalid ciphertext", nil)
	}
	plain, err := aesGCMOpen(aesKey, iv, ct, []byte(aad))
	if err != nil {
		return "", middleware.BadRequest("aes-gcm decrypt failed", nil)
	}
	return string(plain), nil
}
