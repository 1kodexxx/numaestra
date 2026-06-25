package robokassa

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
)

// buildRefundJWT формирует JWT для POST /RefundService/Refund/Create.
// Payload — компактный JSON только с OpKey (полный возврат без фискального чека).
func buildRefundJWT(opKey, password3 string) (string, error) {
	if opKey == "" {
		return "", fmt.Errorf("пустой OpKey")
	}
	if password3 == "" {
		return "", fmt.Errorf("не задан Password3 для API возвратов")
	}

	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"HS256","typ":"JWT"}`))
	payloadBytes, err := json.Marshal(map[string]string{"OpKey": opKey})
	if err != nil {
		return "", fmt.Errorf("сериализация payload: %w", err)
	}
	payload := base64.RawURLEncoding.EncodeToString(payloadBytes)

	signingInput := header + "." + payload
	mac := hmac.New(sha256.New, []byte(password3))
	mac.Write([]byte(signingInput))
	sig := base64.RawURLEncoding.EncodeToString(mac.Sum(nil))

	return signingInput + "." + sig, nil
}
