package encryption

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
)

// Cipher encrypts and decrypts strings for at-rest protection of sensitive fields.
type Cipher interface {
	Encrypt(plaintext string) (string, error)
	Decrypt(ciphertext string) (string, error)
}

type aesCipher struct {
	key []byte
}

// New returns an AES-256-GCM Cipher. key must be exactly 32 bytes.
func New(key []byte) (Cipher, error) {
	if len(key) != 32 {
		return nil, fmt.Errorf("encryption: ключ должен быть 32 байта (AES-256), получено %d", len(key))
	}
	k := make([]byte, 32)
	copy(k, key)
	return &aesCipher{key: k}, nil
}

// Encrypt encrypts plaintext using AES-256-GCM with a random 12-byte nonce.
// Output is base64-encoded: nonce || ciphertext+tag.
func (c *aesCipher) Encrypt(plaintext string) (string, error) {
	block, err := aes.NewCipher(c.key)
	if err != nil {
		return "", fmt.Errorf("encryption: создание блочного шифра: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("encryption: создание GCM: %w", err)
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", fmt.Errorf("encryption: генерация nonce: %w", err)
	}
	sealed := gcm.Seal(nonce, nonce, []byte(plaintext), nil)
	return base64.StdEncoding.EncodeToString(sealed), nil
}

// Decrypt decrypts a base64-encoded AES-256-GCM ciphertext produced by Encrypt.
func (c *aesCipher) Decrypt(ciphertext string) (string, error) {
	raw, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		return "", fmt.Errorf("encryption: декодирование base64: %w", err)
	}
	block, err := aes.NewCipher(c.key)
	if err != nil {
		return "", fmt.Errorf("encryption: создание блочного шифра: %w", err)
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", fmt.Errorf("encryption: создание GCM: %w", err)
	}
	if len(raw) < gcm.NonceSize() {
		return "", errors.New("encryption: шифртекст слишком короткий")
	}
	nonce, ct := raw[:gcm.NonceSize()], raw[gcm.NonceSize():]
	plaintext, err := gcm.Open(nil, nonce, ct, nil)
	if err != nil {
		return "", fmt.Errorf("encryption: расшифровка (неверный ключ или повреждён шифртекст): %w", err)
	}
	return string(plaintext), nil
}
