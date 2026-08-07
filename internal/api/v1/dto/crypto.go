package dto

// ========== 加密工具模块（API设计.md §11 crypto 路由） ==========

// CryptoConfigDTO 传输加密配置（GET /crypto/config）。
type CryptoConfigDTO struct {
	TransportEncryption string `json:"transport_encryption"` // "aes-256-gcm" | 等
}

// PublicKeyDTO RSA-2048 公钥（GET /crypto/public-key）。
// 前端用该公钥做混合加密：随机 AES key 用 RSA 包（wrapped_key），
// 业务密文用 AES-GCM（iv+ciphertext），aad 参与认证（防重放/防篡改）。
type PublicKeyDTO struct {
	PublicKey string `json:"public_key"` // PEM
	Algorithm string `json:"algorithm"`  // "RSA-OAEP-256"
}

// DecryptRequest 服务端解密请求（POST /crypto/decrypt）。
// 必须携带认证签名头（RequireSignature 已校验），ts 参与 aad 防重放。
type DecryptRequest struct {
	WrappedKey  string `json:"wrapped_key" binding:"required"` // RSA-OAEP 包裹的 AES key（base64）
	IV          string `json:"iv" binding:"required"`          // AES-GCM nonce（base64）
	Ciphertext  string `json:"ciphertext" binding:"required"`  // 密文（base64）
	AAD         string `json:"aad" binding:"required"`         // 认证数据：ts+path，防密文跨端点重放
	TS          string `json:"ts" binding:"required"`          // 毫秒时间戳（校验 ±5min）
}

// DecryptResponse 解密结果。
type DecryptResponse struct {
	Plaintext string `json:"plaintext"`
}
