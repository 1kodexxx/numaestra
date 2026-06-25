package robokassa

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const (
	refundCreateBaseURL = "https://services.robokassa.ru/RefundService/Refund/Create"
	refundStateBaseURL  = "https://services.robokassa.ru/RefundService/Refund/GetState"
)

type refundCreateResponse struct {
	Success   bool   `json:"success"`
	Message   string `json:"message"`
	RequestID string `json:"requestId"`
}

type refundStateResponse struct {
	RequestID string  `json:"requestId"`
	Amount    float64 `json:"amount"`
	Label     string  `json:"label"`
	Message   string  `json:"message"`
}

func (c *Client) createRefund(ctx context.Context, opKey string) (string, error) {
	token, err := buildRefundJWT(opKey, c.password3)
	if err != nil {
		return "", err
	}

	endpoint := c.refundCreateURL
	if endpoint == "" {
		endpoint = refundCreateBaseURL
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(token))
	if err != nil {
		return "", fmt.Errorf("refund create: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/jwt")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("refund create: http request: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("refund create: HTTP %d: %s", resp.StatusCode, sanitizeAPIBody(string(body)))
	}

	var parsed refundCreateResponse
	if err := json.Unmarshal(body, &parsed); err != nil {
		return "", fmt.Errorf("refund create: разбор ответа: %w (%s)", err, sanitizeAPIBody(string(body)))
	}
	if !parsed.Success || parsed.RequestID == "" {
		msg := strings.TrimSpace(parsed.Message)
		if msg == "" {
			msg = "неизвестная ошибка"
		}
		return "", fmt.Errorf("refund create: %s", msg)
	}
	return parsed.RequestID, nil
}

func (c *Client) waitRefundFinished(ctx context.Context, requestID string) error {
	endpoint := c.refundStateURL
	if endpoint == "" {
		endpoint = refundStateBaseURL
	}

	deadline := time.Now().Add(45 * time.Second)
	for {
		if err := ctx.Err(); err != nil {
			return err
		}
		if time.Now().After(deadline) {
			return fmt.Errorf("таймаут ожидания возврата (requestId=%s)", requestID)
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint+"?id="+requestID, nil)
		if err != nil {
			return fmt.Errorf("refund state: build request: %w", err)
		}

		resp, err := c.httpClient.Do(req)
		if err != nil {
			return fmt.Errorf("refund state: http request: %w", err)
		}

		body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
		resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			return fmt.Errorf("refund state: HTTP %d: %s", resp.StatusCode, sanitizeAPIBody(string(body)))
		}

		var parsed refundStateResponse
		if err := json.Unmarshal(body, &parsed); err != nil {
			return fmt.Errorf("refund state: разбор ответа: %w", err)
		}

		switch parsed.Label {
		case "finished":
			return nil
		case "canceled":
			return fmt.Errorf("возврат отменён в Robokassa (requestId=%s)", requestID)
		case "processing", "":
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-time.After(3 * time.Second):
				continue
			}
		default:
			return fmt.Errorf("неизвестный статус возврата %q (requestId=%s)", parsed.Label, requestID)
		}
	}
}

func sanitizeAPIBody(body string) string {
	body = strings.TrimSpace(body)
	lower := strings.ToLower(body)
	if strings.HasPrefix(lower, "<!doctype") || strings.HasPrefix(lower, "<html") {
		return "сервер вернул HTML вместо API (возможно устаревший endpoint)"
	}
	if len(body) > 200 {
		return body[:200] + "..."
	}
	return body
}
