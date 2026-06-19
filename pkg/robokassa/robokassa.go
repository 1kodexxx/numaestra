// Package robokassa реализует интеграцию с платёжной системой Robokassa.
//
// Документация по алгоритму подписи:
// https://docs.robokassa.ru/en/#3526
//
// Алгоритм подписи:
//   - Для генерации ссылки на оплату (InitPayment):
//     MD5(MerchantLogin:OutSum:InvId:Password1)
//   - Для проверки вебхука ResultURL:
//     MD5(OutSum:InvId:Password2)
package robokassa

import (
	"context"
	"crypto/md5"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

const (
	paymentBaseURL = "https://auth.robokassa.ru/Merchant/Index.aspx"
	refundBaseURL  = "https://auth.robokassa.ru/Merchant/Refund/Submit"
)

// Client инкапсулирует учётные данные мерчанта и методы работы с Robokassa.
type Client struct {
	merchantLogin string
	password1     string // для генерации ссылки оплаты
	password2     string // для проверки подписи вебхука
	isTest        bool
}

// New создаёт клиент Robokassa с переданными учётными данными.
func New(merchantLogin, password1, password2 string, isTest bool) *Client {
	return &Client{
		merchantLogin: merchantLogin,
		password1:     password1,
		password2:     password2,
		isTest:        isTest,
	}
}

// PaymentURL формирует ссылку для перенаправления клиента на страницу оплаты.
// outSum — сумма в рублях с двумя знаками после запятой (например "1500.00").
// invID — уникальный номер счёта (InvId).
// description — описание заказа, отображаемое на странице оплаты.
func (c *Client) PaymentURL(outSum string, invID int64, description string) string {
	invIDStr := fmt.Sprintf("%d", invID)
	sig := c.signPayment(outSum, invIDStr)

	isTest := "0"
	if c.isTest {
		isTest = "1"
	}

	params := url.Values{}
	params.Set("MerchantLogin", c.merchantLogin)
	params.Set("OutSum", outSum)
	params.Set("InvId", invIDStr)
	params.Set("Description", description)
	params.Set("SignatureValue", sig)
	params.Set("IsTest", isTest)

	return paymentBaseURL + "?" + params.Encode()
}

// VerifyWebhook проверяет подпись входящего уведомления от Robokassa (ResultURL).
// Возвращает true, если подпись совпадает с ожидаемой.
// outSum и invID берутся из параметров POST-запроса от Robokassa.
func (c *Client) VerifyWebhook(outSum, invID, signature string) bool {
	expected := c.signWebhook(outSum, invID)
	// Robokassa может прислать подпись в любом регистре, поэтому нормализуем оба
	// значения к верхнему регистру. Сравнение делаем за константное время через
	// subtle.ConstantTimeCompare, чтобы исключить утечку информации о подписи
	// по времени выполнения (timing attack).
	got := strings.ToUpper(signature)
	return subtle.ConstantTimeCompare([]byte(got), []byte(expected)) == 1
}

// FormatAmount переводит сумму из копеек в строку рублей, которую принимает Robokassa.
// Robokassa требует формат с двумя знаками после запятой: "1500.00", а не "1500".
// Используется целочисленная арифметика, без float — это исключает ошибки округления
// на больших суммах и гарантирует точное совпадение формата с ParseAmountKopecks.
func FormatAmount(amountKopecks int64) string {
	sign := ""
	if amountKopecks < 0 {
		sign = "-"
		amountKopecks = -amountKopecks
	}
	return fmt.Sprintf("%s%d.%02d", sign, amountKopecks/100, amountKopecks%100)
}

// ParseAmountKopecks разбирает сумму в рублях из вебхука Robokassa (например "1500.00"
// или "1500") в целочисленные копейки. Парсинг идёт по строке без float, чтобы
// исключить ошибки округления при сверке оплаченной суммы с суммой заказа.
func ParseAmountKopecks(outSum string) (int64, error) {
	s := strings.TrimSpace(outSum)
	if s == "" {
		return 0, fmt.Errorf("пустая сумма")
	}
	// Robokassa использует точку как десятичный разделитель.
	rubStr, kopStr, hasFraction := strings.Cut(s, ".")
	if rubStr == "" {
		rubStr = "0"
	}

	rub, err := strconv.ParseInt(rubStr, 10, 64)
	if err != nil || rub < 0 {
		return 0, fmt.Errorf("некорректная рублёвая часть суммы %q", outSum)
	}

	var kop int64
	if hasFraction {
		switch len(kopStr) {
		case 0:
			kop = 0
		case 1:
			d, perr := strconv.ParseInt(kopStr, 10, 64)
			if perr != nil || d < 0 {
				return 0, fmt.Errorf("некорректная копеечная часть суммы %q", outSum)
			}
			kop = d * 10
		case 2:
			d, perr := strconv.ParseInt(kopStr, 10, 64)
			if perr != nil || d < 0 {
				return 0, fmt.Errorf("некорректная копеечная часть суммы %q", outSum)
			}
			kop = d
		default:
			return 0, fmt.Errorf("слишком много знаков после запятой в сумме %q", outSum)
		}
	}

	return rub*100 + kop, nil
}

// Refund инициирует возврат платежа через Robokassa API.
// outSum — сумма возврата в рублях (формат "1500.00"), invID — номер счёта.
// Подпись: MD5(OutSum:InvId:Password1) — согласно документации Robokassa.
// https://docs.robokassa.ru/en/#3529
func (c *Client) Refund(ctx context.Context, outSum string, invID int64) error {
	invIDStr := strconv.FormatInt(invID, 10)
	sig := upperMD5(fmt.Sprintf("%s:%s:%s", outSum, invIDStr, c.password1))

	params := url.Values{}
	params.Set("MrchLogin", c.merchantLogin)
	params.Set("InvId", invIDStr)
	params.Set("OutSum", outSum)
	params.Set("Signature", sig)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, refundBaseURL, strings.NewReader(params.Encode()))
	if err != nil {
		return fmt.Errorf("robokassa refund: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("robokassa refund: http request: %w", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("robokassa refund: HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	// Robokassa возвращает "OK{InvId}" при успехе и "ERROR{код}" при ошибке.
	bodyStr := strings.TrimSpace(string(body))
	if !strings.HasPrefix(bodyStr, "OK") {
		return fmt.Errorf("robokassa refund: отказ от сервера: %s", bodyStr)
	}
	return nil
}

// signPayment вычисляет MD5-подпись для генерации ссылки оплаты.
// Формула: MD5(MerchantLogin:OutSum:InvId:Password1)
func (c *Client) signPayment(outSum, invID string) string {
	raw := fmt.Sprintf("%s:%s:%s:%s", c.merchantLogin, outSum, invID, c.password1)
	return upperMD5(raw)
}

// signWebhook вычисляет MD5-подпись для верификации вебхука ResultURL.
// Формула: MD5(OutSum:InvId:Password2)
func (c *Client) signWebhook(outSum, invID string) string {
	raw := fmt.Sprintf("%s:%s:%s", outSum, invID, c.password2)
	return upperMD5(raw)
}

func upperMD5(s string) string {
	h := md5.Sum([]byte(s))
	return strings.ToUpper(hex.EncodeToString(h[:]))
}
