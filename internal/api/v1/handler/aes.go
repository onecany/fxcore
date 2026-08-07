package handler

import (
	"crypto/aes"
	"crypto/cipher"
	"errors"
)

// aesGCMOpen AES-GCM 解密（供 crypto handler 使用）。
func aesGCMOpen(key, iv, ciphertext, aad []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	if len(iv) != gcm.NonceSize() {
		return nil, errors.New("invalid nonce size")
	}
	return gcm.Open(nil, iv, ciphertext, aad)
}
