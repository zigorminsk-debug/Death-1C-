//go:build !windows

package config

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"fmt"
	"io"
	"os"
	"strings"
)

const aesPrefix = "aes:"

func deriveLocalKey() []byte {
	host, _ := os.Hostname()
	user := os.Getenv("USER")
	sum := sha256.Sum256([]byte("death1c-local-key|" + host + "|" + user))
	return sum[:]
}

func encryptSecret(plain string) (string, error) {
	if plain == "" {
		return "", nil
	}
	block, err := aes.NewCipher(deriveLocalKey())
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
	sealed := gcm.Seal(nonce, nonce, []byte(plain), nil)
	return aesPrefix + base64.StdEncoding.EncodeToString(sealed), nil
}

func decryptSecret(encoded string) (string, error) {
	if encoded == "" {
		return "", nil
	}
	rawB64 := strings.TrimPrefix(encoded, aesPrefix)
	data, err := base64.StdEncoding.DecodeString(rawB64)
	if err != nil {
		return "", fmt.Errorf("ошибка декодирования пароля: %w", err)
	}
	block, err := aes.NewCipher(deriveLocalKey())
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	if len(data) < gcm.NonceSize() {
		return "", fmt.Errorf("повреждённые данные пароля")
	}
	nonce, cipherBytes := data[:gcm.NonceSize()], data[gcm.NonceSize():]
	plain, err := gcm.Open(nil, nonce, cipherBytes, nil)
	if err != nil {
		return "", fmt.Errorf("не удалось расшифровать пароль: %w", err)
	}
	return string(plain), nil
}
