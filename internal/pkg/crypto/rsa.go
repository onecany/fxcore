// Package crypto 提供 API Key 的 RSA 加密存储能力（API设计.md 6.2）。
// 使用 RSA-OAEP(SHA-256)，密钥 2048 位。
package crypto

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"strings"
)

// KeyManager 持有服务端 RSA 私钥，负责 API Key 的加密与解密。
// 密钥来源优先级：RSA_PRIVATE_KEY 环境变量（PEM 或 base64(PEM)）> 启动时生成（仅存内存）。
type KeyManager struct {
	priv *rsa.PrivateKey
}

// NewKeyManager 加载或生成 RSA 密钥对。
// privateKeyPEM 为空时生成新的 2048 位密钥。
func NewKeyManager(privateKeyPEM string) (*KeyManager, error) {
	if privateKeyPEM != "" {
		der, err := decodeKeyMaterial(privateKeyPEM)
		if err != nil {
			return nil, fmt.Errorf("parse RSA_PRIVATE_KEY: %w", err)
		}
		key, err := x509.ParsePKCS8PrivateKey(der)
		if err == nil {
			rk, ok := key.(*rsa.PrivateKey)
			if !ok {
				return nil, errors.New("RSA_PRIVATE_KEY is not an RSA key")
			}
			return &KeyManager{priv: rk}, nil
		}
		// 兼容 PKCS#1 格式
		rk, err2 := x509.ParsePKCS1PrivateKey(der)
		if err2 != nil {
			return nil, fmt.Errorf("parse private key (PKCS8: %v; PKCS1: %v)", err, err2)
		}
		return &KeyManager{priv: rk}, nil
	}

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, fmt.Errorf("generate RSA key: %w", err)
	}
	return &KeyManager{priv: key}, nil
}

// decodeKeyMaterial 解码 PEM 或 base64(PEM) 私钥材料。
func decodeKeyMaterial(raw string) ([]byte, error) {
	raw = strings.TrimSpace(raw)
	if strings.Contains(raw, "-----BEGIN") {
		block, _ := pem.Decode([]byte(raw))
		if block == nil {
			return nil, errors.New("invalid PEM block")
		}
		return block.Bytes, nil
	}
	// 尝试 base64 编码的 PEM
	der, err := base64.StdEncoding.DecodeString(raw)
	if err != nil {
		return nil, errors.New("key material is neither PEM nor base64")
	}
	if strings.Contains(string(der), "-----BEGIN") {
		block, _ := pem.Decode(der)
		if block == nil {
			return nil, errors.New("invalid PEM block after base64 decode")
		}
		return block.Bytes, nil
	}
	return der, nil
}

// Encrypt 使用 OAEP-SHA256 加密，返回 base64 密文。
func (km *KeyManager) Encrypt(plain []byte) (string, error) {
	if km == nil || km.priv == nil {
		return "", errors.New("key manager not initialized")
	}
	cipher, err := rsa.EncryptOAEP(sha256.New(), rand.Reader, &km.priv.PublicKey, plain, nil)
	if err != nil {
		return "", fmt.Errorf("rsa encrypt: %w", err)
	}
	return base64.StdEncoding.EncodeToString(cipher), nil
}

// Decrypt 解密 base64 密文。
func (km *KeyManager) Decrypt(enc string) ([]byte, error) {
	if km == nil || km.priv == nil {
		return nil, errors.New("key manager not initialized")
	}
	cipher, err := base64.StdEncoding.DecodeString(enc)
	if err != nil {
		return nil, fmt.Errorf("decode ciphertext: %w", err)
	}
	plain, err := rsa.DecryptOAEP(sha256.New(), rand.Reader, km.priv, cipher, nil)
	if err != nil {
		return nil, fmt.Errorf("rsa decrypt: %w", err)
	}
	return plain, nil
}

// PublicKeyPEM 导出公钥（PEM），用于密钥轮换/审计。
func (km *KeyManager) PublicKeyPEM() string {
	if km == nil || km.priv == nil {
		return ""
	}
	der, err := x509.MarshalPKIXPublicKey(&km.priv.PublicKey)
	if err != nil {
		return ""
	}
	return string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der}))
}

// KeySize 返回密钥位数（供日志/健康检查）。
func (km *KeyManager) KeySize() int {
	if km == nil || km.priv == nil {
		return 0
	}
	return km.priv.Size() * 8
}
