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
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	paymentBaseURL = "https://auth.robokassa.ru/Merchant/Index.aspx"
)

// Client инкапсулирует учётные данные мерчанта и методы работы с Robokassa.
type Client struct {
	merchantLogin   string
	password1       string // для генерации ссылки оплаты
	password2       string // для проверки подписи вебхука и OpStateExt
	password3       string // для JWT API возвратов
	isTest          bool
	testAutoPay     bool // для тестового режима: IsInvoicePaid/GetPaidAmountKopecks возвращают true
	httpClient      *http.Client
	opStateURL      string // переопределяется в тестах
	refundCreateURL string // переопределяется в тестах
	refundStateURL  string // переопределяется в тестах

	// refundMu/pendingRefunds — защита от повторного создания возврата в Robokassa.
	// Refund() — составная операция (OpKey → Create → GetState); если она прервётся
	// после успешного Create (таймаут ожидания, обрыв сети), повторный вызов Refund()
	// для того же InvId не должен создавать ВТОРОЙ реальный возврат. Кэшируем
	// requestId, выданный Create, и при повторном вызове сразу переходим к ожиданию
	// существующего возврата. Кэш только в памяти процесса: после перезапуска сервиса
	// защита сбрасывается — это осознанный компромисс ради простоты, дублирующий
	// возврат после рестарта — на порядки менее вероятный сценарий, чем повторный
	// клик администратора сразу после таймаута.
	refundMu       sync.Mutex
	pendingRefunds map[int64]string
}

// New создаёт клиент Robokassa с переданными учётными данными.
// password3 нужен для возвратов через Refund API (генерируется отдельно в кабинете).
func New(merchantLogin, password1, password2, password3 string, isTest bool) *Client {
	return &Client{
		merchantLogin:  merchantLogin,
		password1:      password1,
		password2:      password2,
		password3:      password3,
		isTest:         isTest,
		httpClient:     &http.Client{Timeout: 30 * time.Second},
		pendingRefunds: make(map[int64]string),
	}
}

// WithTestAutoPay включает режим автоматического подтверждения оплаты в тестовой среде.
// Активируется через ROBOKASSA_TEST_AUTO_PAY=true только при IsTest=true.
func (c *Client) WithTestAutoPay() *Client {
	c.testAutoPay = true
	return c
}

// IsTestAutoPay сообщает, включён ли тестовый автоплатёж.
// Используется в обработчике sync-payment, чтобы пропустить реальный запрос к Robokassa.
func (c *Client) IsTestAutoPay() bool {
	return c.isTest && c.testAutoPay
}

// WithTestHTTP подключает httptest-сервер к XML/Refund API (используется в HTTP-тестах).
func (c *Client) WithTestHTTP(httpClient *http.Client, opStateURL string) *Client {
	if httpClient != nil {
		c.httpClient = httpClient
	}
	if opStateURL != "" {
		c.opStateURL = opStateURL
	}
	return c
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

// Refund инициирует полный возврат платежа через Robokassa Refund API v2.
// invID — номер счёта (InvId). outSum сохранён в сигнатуре для совместимости,
// но для полного возврата не передаётся в API.
// Требует Password3 и боевой платёж (OpStateExt не работает для тестовых оплат).
// https://docs.robokassa.ru/ru/refund-api
func (c *Client) Refund(ctx context.Context, outSum string, invID int64) error {
	_ = outSum

	if c.password3 == "" {
		return fmt.Errorf("не задан ROBOKASSA_PASS3 — сгенерируйте Password3 в кабинете Robokassa")
	}

	// Если для этого InvId уже есть незавершённый возврат (предыдущий вызов прервался
	// после Create, например по таймауту GetState) — переиспользуем его requestId
	// вместо того, чтобы создавать новый возврат повторно.
	requestID := c.getPendingRefund(invID)
	if requestID == "" {
		opKey, err := c.fetchOpKey(ctx, invID)
		if err != nil {
			return fmt.Errorf("получение OpKey: %w", err)
		}

		requestID, err = c.createRefund(ctx, opKey)
		if err != nil {
			return err
		}
		c.setPendingRefund(invID, requestID)
	}

	if err := c.waitRefundFinished(ctx, requestID); err != nil {
		// requestID остаётся в кэше — повторный вызов Refund() для этого InvId
		// продолжит ожидание уже созданного возврата, а не создаст новый.
		return err
	}

	c.clearPendingRefund(invID)
	return nil
}

func (c *Client) getPendingRefund(invID int64) string {
	c.refundMu.Lock()
	defer c.refundMu.Unlock()
	return c.pendingRefunds[invID]
}

func (c *Client) setPendingRefund(invID int64, requestID string) {
	c.refundMu.Lock()
	defer c.refundMu.Unlock()
	c.pendingRefunds[invID] = requestID
}

func (c *Client) clearPendingRefund(invID int64) {
	c.refundMu.Lock()
	defer c.refundMu.Unlock()
	delete(c.pendingRefunds, invID)
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
