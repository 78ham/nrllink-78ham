package main

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"strings"
)

// Magic header for encrypted license files (8 bytes)
var licenseMagic = []byte("NRLLIC01")

var (
	licenseAESKey  []byte
	fieldAESKey    []byte
	fieldPepperKey []byte
)

// deriveKey 将任意长度的密钥字符串转换为标准的 32 字节 AES-256 密钥
func deriveKey(keyStr string) []byte {
	if keyStr == "" {
		return nil
	}
	// 如果是 64 位十六进制字符串，直接解码为 32 字节
	if len(keyStr) == 64 {
		if b, err := hex.DecodeString(keyStr); err == nil {
			return b
		}
	}
	// 否则进行 SHA-256 哈希派生为 32 字节密钥
	h := sha256.Sum256([]byte(keyStr))
	return h[:]
}

// encryptAESGCM 对二进制字节流执行 AES-256-GCM 加密，包含魔数头部和 12 字节随机 Nonce
func encryptAESGCM(key []byte, plaintext []byte) ([]byte, error) {
	if len(key) != 32 {
		return nil, fmt.Errorf("invalid key length for AES-256: expected 32, got %d", len(key))
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, err
	}

	ciphertext := gcm.Seal(nil, nonce, plaintext, nil)

	// 打包格式：[8字节Magic][12字节Nonce][密文+16字节Tag]
	result := make([]byte, 0, len(licenseMagic)+len(nonce)+len(ciphertext))
	result = append(result, licenseMagic...)
	result = append(result, nonce...)
	result = append(result, ciphertext...)
	return result, nil
}

// decryptAESGCM 对由 encryptAESGCM 加密的字节流执行解密
func decryptAESGCM(key []byte, data []byte) ([]byte, error) {
	if len(key) != 32 {
		return nil, fmt.Errorf("invalid key length for AES-256: expected 32, got %d", len(key))
	}
	headerLen := len(licenseMagic) + 12
	if len(data) < headerLen {
		return nil, errors.New("ciphertext too short")
	}

	// 检查头部魔数
	if string(data[:len(licenseMagic)]) != string(licenseMagic) {
		return nil, errors.New("invalid license file magic header")
	}

	nonce := data[len(licenseMagic):headerLen]
	ciphertext := data[headerLen:]

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	return gcm.Open(nil, nonce, ciphertext, nil)
}

// isEncryptedLicense 检查二进制数据是否携带加密执照魔数头
func isEncryptedLicense(data []byte) bool {
	if len(data) < len(licenseMagic) {
		return false
	}
	return string(data[:len(licenseMagic)]) == string(licenseMagic)
}

// encryptField 对文本字段进行 AES-GCM 加密并 Base64 编码，增加 "enc:" 前缀标识
func encryptField(key []byte, plaintext string) (string, error) {
	if len(key) != 32 || plaintext == "" {
		return plaintext, nil
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	ciphertext := gcm.Seal(nil, nonce, []byte(plaintext), nil)
	combined := append(nonce, ciphertext...)
	return "enc:" + base64.StdEncoding.EncodeToString(combined), nil
}

// decryptField 对带 "enc:" 前缀的 Base64 密文进行解密；若不是密文则原样返回明文
func decryptField(key []byte, val string) string {
	if !strings.HasPrefix(val, "enc:") || len(key) != 32 {
		return val
	}
	b64 := strings.TrimPrefix(val, "enc:")
	combined, err := base64.StdEncoding.DecodeString(b64)
	if err != nil || len(combined) < 12 {
		return val
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return val
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return val
	}

	nonce := combined[:gcm.NonceSize()]
	ciphertext := combined[gcm.NonceSize():]

	plain, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return val
	}
	return string(plain)
}

// hashIndex 对索引字段（如手机号）计算不可逆的 HMAC-SHA256 哈希值，用于密文检索
func hashIndex(pepper []byte, value string) string {
	clean := strings.TrimSpace(value)
	if clean == "" {
		return ""
	}
	if len(pepper) == 0 {
		pepper = []byte("nrllink_default_pepper_key_2026")
	}
	mac := hmac.New(sha256.New, pepper)
	mac.Write([]byte(strings.ToLower(clean)))
	return hex.EncodeToString(mac.Sum(nil))
}
