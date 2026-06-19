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
	"crypto/md5"
	"encoding/hex"
	"fmt"
	"net/url"
	"strings"
)

const paymentBaseURL = "https://auth.robokassa.ru/Merchant/Index.aspx"

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
	return strings.EqualFold(signature, expected)
}

// FormatAmount переводит сумму из копеек в строку рублей, которую принимает Robokassa.
// Robokassa требует формат с двумя знаками после запятой: "1500.00", а не "1500".
func FormatAmount(amountKopecks int64) string {
	return fmt.Sprintf("%.2f", float64(amountKopecks)/100)
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
