// Package adminsession реализует подписанные сессионные токены админки без
// сервера состояний: проверка подписи и срока действия не требует обращения к
// БД/Redis, что упрощает горизонтальное масштабирование API (любой инстанс с
// тем же секретом проверит токен любого другого).
//
// Формат токена: base64url(login) + "." + base64url(unix-таймстамп истечения)
// + "." + base64url(HMAC-SHA256 первых двух частей).
package adminsession

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"strconv"
	"strings"
	"time"
)

// Issue создаёт подписанный токен сессии для login со сроком действия ttl.
func Issue(secret []byte, login string, ttl time.Duration) string {
	exp := time.Now().Add(ttl).Unix()
	payload := encodePart(login) + "." + encodePart(strconv.FormatInt(exp, 10))
	return payload + "." + base64.RawURLEncoding.EncodeToString(sign(secret, payload))
}

// Verify проверяет подпись и срок действия токена. ok=false при любой
// проблеме: неверный формат, неверная подпись или истёкший срок действия —
// детали не различаются намеренно, чтобы не давать атакующему лишней информации.
func Verify(secret []byte, token string) (login string, ok bool) {
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return "", false
	}

	payload := parts[0] + "." + parts[1]
	gotSig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return "", false
	}
	if !hmac.Equal(gotSig, sign(secret, payload)) {
		return "", false
	}

	loginBytes, err := base64.RawURLEncoding.DecodeString(parts[0])
	if err != nil {
		return "", false
	}
	expBytes, err := base64.RawURLEncoding.DecodeString(parts[1])
	if err != nil {
		return "", false
	}
	exp, err := strconv.ParseInt(string(expBytes), 10, 64)
	if err != nil {
		return "", false
	}
	if time.Now().Unix() > exp {
		return "", false
	}
	return string(loginBytes), true
}

func sign(secret []byte, payload string) []byte {
	mac := hmac.New(sha256.New, secret)
	mac.Write([]byte(payload))
	return mac.Sum(nil)
}

func encodePart(s string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(s))
}
