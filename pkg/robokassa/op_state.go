package robokassa

import (
	"context"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

const opStateBaseURL = "https://auth.robokassa.ru/Merchant/WebService/Service.asmx/OpStateExt"

// StateCodePaid — успешная оплата по документации OpStateExt Robokassa.
const StateCodePaid = 100

// ErrOperationNotFound — OpStateExt вернул Result.Code=3: операции с таким InvId
// у Robokassa нет. Для pending-заказа это штатное «ещё не оплачено» (клиент не
// начинал/не завершил оплату), а не сбой. Для возвратов (fetchOpKey) — реальная
// ошибка: нельзя вернуть несуществующий платёж.
var ErrOperationNotFound = errors.New("операция не найдена в Robokassa")

// operationStateResponse — подмножество XML-ответа OpStateExt.
type operationStateResponse struct {
	Result struct {
		Code int `xml:"Code"`
	} `xml:"Result"`
	State struct {
		Code int `xml:"Code"`
	} `xml:"State"`
	Info struct {
		OutSum string `xml:"OutSum"`
		OpKey  string `xml:"OpKey"`
	} `xml:"Info"`
}

// IsInvoicePaid проверяет в Robokassa, что счёт оплачен (State.Code = 100).
// Работает только в боевом режиме; для тестовых платежей OpStateExt недоступен.
// «Операция не найдена» (клиент не платил) — не ошибка, а «не оплачено».
func (c *Client) IsInvoicePaid(ctx context.Context, invID int64) (bool, error) {
	if c.isTest {
		return false, nil
	}
	state, err := c.fetchOperationState(ctx, invID)
	if err != nil {
		if errors.Is(err, ErrOperationNotFound) {
			return false, nil
		}
		return false, err
	}
	return state.State.Code == StateCodePaid, nil
}

// GetPaidAmountKopecks проверяет статус счёта через OpStateExt и возвращает
// фактически списанную сумму в копейках. Если счёт не оплачен (в т.ч. операции
// ещё нет в Robokassa) — возвращает (0, false, nil). Работает только в боевом режиме.
func (c *Client) GetPaidAmountKopecks(ctx context.Context, invID int64) (kopecks int64, paid bool, err error) {
	if c.isTest {
		return 0, false, nil
	}
	state, err := c.fetchOperationState(ctx, invID)
	if err != nil {
		if errors.Is(err, ErrOperationNotFound) {
			return 0, false, nil
		}
		return 0, false, err
	}
	if state.State.Code != StateCodePaid {
		return 0, false, nil
	}
	outSum := strings.TrimSpace(state.Info.OutSum)
	if outSum == "" {
		return 0, false, fmt.Errorf("opstate: OutSum отсутствует в ответе для InvoiceID=%d", invID)
	}
	// Парсим строкой, без float: int64(rubles*100) даёт ошибку округления на копейку
	// для нецелых сумм (например 1999.99 → 199998 вместо 199999), из-за чего сверка
	// с суммой заказа ложно бы не совпала. ParseAmountKopecks делает то же, что и
	// разбор вебхука, — гарантируя идентичную сумму на обоих путях подтверждения.
	kopecks, err = ParseAmountKopecks(strings.ReplaceAll(outSum, ",", "."))
	if err != nil {
		return 0, false, fmt.Errorf("opstate: не удалось разобрать OutSum %q: %w", outSum, err)
	}
	return kopecks, true, nil
}

func (c *Client) fetchOperationState(ctx context.Context, invID int64) (*operationStateResponse, error) {
	invIDStr := strconv.FormatInt(invID, 10)
	sig := upperMD5(fmt.Sprintf("%s:%s:%s", c.merchantLogin, invIDStr, c.password2))

	params := url.Values{}
	params.Set("MerchantLogin", c.merchantLogin)
	params.Set("InvoiceID", invIDStr)
	params.Set("Signature", sig)

	endpoint := c.opStateURL
	if endpoint == "" {
		endpoint = opStateBaseURL
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint+"?"+params.Encode(), nil)
	if err != nil {
		return nil, fmt.Errorf("opstate: build request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("opstate: http request: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("opstate: HTTP %d: %s", resp.StatusCode, sanitizeAPIBody(string(body)))
	}

	var state operationStateResponse
	if err := xml.Unmarshal(body, &state); err != nil {
		return nil, fmt.Errorf("opstate: разбор XML: %w", err)
	}

	switch state.Result.Code {
	case 0:
		// ok
	case 3:
		return nil, fmt.Errorf("InvoiceID=%s: %w", invIDStr, ErrOperationNotFound)
	default:
		return nil, fmt.Errorf("opstate: код результата %d", state.Result.Code)
	}

	return &state, nil
}

// fetchOpKey запрашивает OpKey операции по номеру счёта через OpStateExt.
func (c *Client) fetchOpKey(ctx context.Context, invID int64) (string, error) {
	state, err := c.fetchOperationState(ctx, invID)
	if err != nil {
		return "", err
	}
	opKey := strings.TrimSpace(state.Info.OpKey)
	if opKey == "" {
		return "", fmt.Errorf("opstate: пустой OpKey для InvoiceID=%d", invID)
	}
	return opKey, nil
}
