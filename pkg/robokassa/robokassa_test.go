package robokassa

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// Тестовые учётные данные (не боевые).
const (
	testMerchantLogin = "TestMerchant"
	testPassword1     = "TestPassword1"
	testPassword2     = "TestPassword2"
	testPassword3     = "TestPassword3"
)

func newTestClient(isTest bool) *Client {
	return New(testMerchantLogin, testPassword1, testPassword2, testPassword3, isTest)
}

// --- FormatAmount ---

func TestFormatAmount(t *testing.T) {
	tests := []struct {
		kopecks int64
		want    string
	}{
		{150000, "1500.00"},       // ровные рубли
		{150050, "1500.50"},       // с копейками
		{1, "0.01"},               // одна копейка
		{100, "1.00"},             // один рубль
		{99999, "999.99"},         // нестандартная сумма
		{100000000, "1000000.00"}, // большая сумма
	}

	for _, tt := range tests {
		got := FormatAmount(tt.kopecks)
		if got != tt.want {
			t.Errorf("FormatAmount(%d) = %q, хотели %q", tt.kopecks, got, tt.want)
		}
	}
}

// --- signPayment (через PaymentURL) ---

func TestClient_PaymentURL_ContainsRequiredParams(t *testing.T) {
	c := newTestClient(false)
	u := c.PaymentURL("1500.00", 42, "Тестовый заказ", "")

	for _, param := range []string{"MerchantLogin", "OutSum", "InvId", "SignatureValue", "Description"} {
		if !strings.Contains(u, param) {
			t.Errorf("PaymentURL не содержит параметр %q: %s", param, u)
		}
	}
}

func TestClient_PaymentURL_IsTestFlag(t *testing.T) {
	prod := newTestClient(false)
	test := newTestClient(true)

	uProd := prod.PaymentURL("100.00", 1, "Desc", "")
	uTest := test.PaymentURL("100.00", 1, "Desc", "")

	if !strings.Contains(uProd, "IsTest=0") {
		t.Errorf("продакшн URL должен содержать IsTest=0: %s", uProd)
	}
	if !strings.Contains(uTest, "IsTest=1") {
		t.Errorf("тестовый URL должен содержать IsTest=1: %s", uTest)
	}
}

func TestClient_PaymentURL_SignatureIsUpperMD5(t *testing.T) {
	c := newTestClient(false)
	u := c.PaymentURL("1500.00", 7, "Desc", "")

	// Извлекаем SignatureValue из URL
	sig := extractParam(u, "SignatureValue")
	if sig == "" {
		t.Fatal("SignatureValue не найден в URL")
	}
	// MD5 в hex = 32 символа, верхний регистр
	if len(sig) != 32 {
		t.Errorf("SignatureValue должен быть 32 символа, получили %d: %q", len(sig), sig)
	}
	if sig != strings.ToUpper(sig) {
		t.Errorf("SignatureValue должен быть в верхнем регистре, получили: %q", sig)
	}
}

// По умолчанию Receipt выключен — параметр не должен появляться в URL, а подпись
// считается по старой формуле (карта продолжает работать как раньше).
func TestClient_PaymentURL_NoReceiptByDefault(t *testing.T) {
	c := newTestClient(false)
	u := c.PaymentURL("1500.00", 7, "Desc", "a@b.ru")
	if strings.Contains(u, "Receipt=") {
		t.Errorf("без WithReceipt параметр Receipt не должен передаваться: %s", u)
	}
	// Подпись без чека = MD5(MerchantLogin:OutSum:InvId:Password1)
	want := c.signPayment("1500.00", "7", "")
	if got := extractParam(u, "SignatureValue"); got != want {
		t.Errorf("подпись без чека: got %s, want %s", got, want)
	}
}

// С включённым Receipt: параметр в URL двойной url-encode, а подпись включает
// одинарно закодированный Receipt (MerchantLogin:OutSum:InvId:Receipt:Password1).
func TestClient_PaymentURL_ReceiptInSignature(t *testing.T) {
	c := newTestClient(false).WithReceipt(true, "", "none")
	u := c.PaymentURL("1500.00", 7, "Песня", "a@b.ru")

	if !strings.Contains(u, "Receipt=") {
		t.Fatalf("Receipt должен быть в URL: %s", u)
	}
	// В URL Receipt закодирован дважды → содержит %25 (закодированный %).
	rawReceipt := extractParam(u, "Receipt")
	if !strings.Contains(rawReceipt, "%25") {
		t.Errorf("Receipt в URL должен быть двойным url-encode (ожидали %%25): %s", rawReceipt)
	}

	// Подпись должна совпасть с MD5(...:receiptEnc:pass1), где receiptEnc — одинарный encode.
	receiptEnc := buildReceiptEncoded("Песня", 1500, "", "none")
	want := c.signPayment("1500.00", "7", receiptEnc)
	if got := extractParam(u, "SignatureValue"); got != want {
		t.Errorf("подпись с чеком не совпала: got %s, want %s", got, want)
	}
}

// --- VerifyWebhook ---

func TestClient_VerifyWebhook_ValidSignature(t *testing.T) {
	c := newTestClient(false)

	outSum := "1500.00"
	invID := "42"

	// Правильная формула: OutSum:InvId:Password2
	correctSig := c.signWebhook(outSum, invID)
	if !c.VerifyWebhook(outSum, invID, correctSig) {
		t.Error("VerifyWebhook должен принять корректную подпись")
	}
}

func TestClient_VerifyWebhook_InvalidSignature(t *testing.T) {
	c := newTestClient(false)
	if c.VerifyWebhook("1500.00", "42", "DEADBEEF00000000DEADBEEF00000000") {
		t.Error("VerifyWebhook должен отклонить неверную подпись")
	}
}

func TestClient_VerifyWebhook_CaseInsensitive(t *testing.T) {
	c := newTestClient(false)
	sig := c.signWebhook("500.00", "1")

	// Robokassa может прислать подпись в любом регистре
	if !c.VerifyWebhook("500.00", "1", strings.ToLower(sig)) {
		t.Error("VerifyWebhook должен принять подпись в нижнем регистре")
	}
	if !c.VerifyWebhook("500.00", "1", strings.ToUpper(sig)) {
		t.Error("VerifyWebhook должен принять подпись в верхнем регистре")
	}
}

func TestClient_VerifyWebhook_WrongPassword(t *testing.T) {
	c := newTestClient(false)
	// Подпись посчитана с Password1, а проверяем с Password2 — должна упасть
	wrongSig := c.signPayment("1500.00", "42", "")
	if c.VerifyWebhook("1500.00", "42", wrongSig) {
		t.Error("подпись с неверным паролем не должна проходить верификацию")
	}
}

func TestClient_VerifyWebhook_DifferentAmounts(t *testing.T) {
	c := newTestClient(false)
	sig := c.signWebhook("1500.00", "42")
	// Подменяем сумму
	if c.VerifyWebhook("1500.01", "42", sig) {
		t.Error("подпись не должна проходить при изменённой сумме")
	}
}

// --- Согласованность signPayment / PaymentURL ---

func TestClient_PaymentAndWebhook_SameAmount(t *testing.T) {
	// Критический тест: сумма в ссылке оплаты и в верификации вебхука
	// должна быть в одном формате, иначе подписи не совпадут.
	c := newTestClient(false)
	kopecks := int64(150050) // 1500 рублей 50 копеек

	outSum := FormatAmount(kopecks)
	if outSum != "1500.50" {
		t.Fatalf("FormatAmount вернул %q вместо '1500.50'", outSum)
	}

	// Имитируем: сформировали ссылку → Robokassa прислала вебхук с той же суммой
	invID := "99"
	paymentSig := extractParam(c.PaymentURL(outSum, 99, "Desc", ""), "SignatureValue")
	webhookSig := c.signWebhook(outSum, invID)

	// Подписи разные (разные пароли), но формат суммы должен быть одинаковым
	// Проверяем что вебхук с той же суммой проходит верификацию
	if !c.VerifyWebhook(outSum, invID, webhookSig) {
		t.Error("вебхук с суммой из PaymentURL должен проходить верификацию")
	}
	_ = paymentSig // подпись оплаты использует Password1, вебхука — Password2
}

// --- Refund ---

func TestRefund_Success(t *testing.T) {
	const opKey = "0005F891-8CCD-434B-8455-816AFFFDBF37-0VOisWikFF"
	stateCalls := 0
	createCalls := 0
	statusCalls := 0

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "OpStateExt"):
			stateCalls++
			w.Header().Set("Content-Type", "application/xml")
			fmt.Fprintf(w, `<?xml version="1.0"?><OperationStateResponse xmlns="http://merchant.roboxchange.com/WebService/"><Result><Code>0</Code></Result><Info><OpKey>%s</OpKey></Info></OperationStateResponse>`, opKey)
		case strings.HasSuffix(r.URL.Path, "/Refund/Create"):
			createCalls++
			if ct := r.Header.Get("Content-Type"); ct != "application/jwt" {
				t.Errorf("ожидали application/jwt, получили %q", ct)
			}
			body, _ := io.ReadAll(r.Body)
			if len(body) == 0 {
				t.Fatal("пустое тело JWT")
			}
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"success":true,"requestId":"cf15fd52-d2d1-4fc4-b9c0-25310e3bdded"}`)
		case strings.HasSuffix(r.URL.Path, "/Refund/GetState"):
			statusCalls++
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"requestId":"cf15fd52-d2d1-4fc4-b9c0-25310e3bdded","amount":2000,"label":"finished"}`)
		default:
			t.Fatalf("неожиданный путь: %s", r.URL.Path)
		}
	}))
	defer srv.Close()

	c := newTestClient(false)
	c.httpClient = srv.Client()
	c.opStateURL = srv.URL + "/OpStateExt"
	c.refundCreateURL = srv.URL + "/Refund/Create"
	c.refundStateURL = srv.URL + "/Refund/GetState"

	if err := c.Refund(context.Background(), "2000.00", 7); err != nil {
		t.Fatalf("ожидали успех, получили: %v", err)
	}
	if stateCalls != 1 || createCalls != 1 || statusCalls < 1 {
		t.Fatalf("вызовы API: opstate=%d create=%d state=%d", stateCalls, createCalls, statusCalls)
	}
}

// TestRefund_RetryAfterWaitTimeout_DoesNotCreateSecondRefund проверяет защиту от
// дублирующего возврата: если первый вызов Refund() успешно создал возврат в
// Robokassa, но оборвался на ожидании (GetState вернул ошибку), повторный вызов
// для того же InvId должен переиспользовать тот же requestId и НЕ вызывать
// Refund/Create второй раз.
func TestRefund_RetryAfterWaitTimeout_DoesNotCreateSecondRefund(t *testing.T) {
	const opKey = "0005F891-8CCD-434B-8455-816AFFFDBF37-0VOisWikFF"
	createCalls := 0
	stateCallsBeforeFinish := 0
	failState := true

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "OpStateExt"):
			w.Header().Set("Content-Type", "application/xml")
			fmt.Fprintf(w, `<?xml version="1.0"?><OperationStateResponse xmlns="http://merchant.roboxchange.com/WebService/"><Result><Code>0</Code></Result><Info><OpKey>%s</OpKey></Info></OperationStateResponse>`, opKey)
		case strings.HasSuffix(r.URL.Path, "/Refund/Create"):
			createCalls++
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"success":true,"requestId":"cf15fd52-d2d1-4fc4-b9c0-25310e3bdded"}`)
		case strings.HasSuffix(r.URL.Path, "/Refund/GetState"):
			if failState {
				stateCallsBeforeFinish++
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"requestId":"cf15fd52-d2d1-4fc4-b9c0-25310e3bdded","amount":2000,"label":"finished"}`)
		default:
			t.Fatalf("неожиданный путь: %s", r.URL.Path)
		}
	}))
	defer srv.Close()

	c := newTestClient(false)
	c.httpClient = srv.Client()
	c.opStateURL = srv.URL + "/OpStateExt"
	c.refundCreateURL = srv.URL + "/Refund/Create"
	c.refundStateURL = srv.URL + "/Refund/GetState"

	// Первый вызов: Create проходит, GetState падает (имитация обрыва/таймаута ожидания).
	if err := c.Refund(context.Background(), "2000.00", 7); err == nil {
		t.Fatal("ожидали ошибку на этапе ожидания возврата")
	}
	if createCalls != 1 {
		t.Fatalf("ожидали 1 вызов Refund/Create, получили %d", createCalls)
	}
	if stateCallsBeforeFinish == 0 {
		t.Fatal("ожидали хотя бы один неудачный вызов GetState")
	}

	// Повторный вызов для того же InvId: GetState теперь отвечает успехом.
	failState = false
	if err := c.Refund(context.Background(), "2000.00", 7); err != nil {
		t.Fatalf("ожидали успех при повторном вызове, получили: %v", err)
	}

	// Главная проверка: Refund/Create не вызывался повторно — переиспользован requestId.
	if createCalls != 1 {
		t.Fatalf("повторный Refund() создал ВТОРОЙ возврат: createCalls=%d, ожидали 1", createCalls)
	}
}

func TestRefund_MissingPassword3(t *testing.T) {
	c := New(testMerchantLogin, testPassword1, testPassword2, "", false)
	err := c.Refund(context.Background(), "2000.00", 7)
	if err == nil || !strings.Contains(err.Error(), "ROBOKASSA_PASS3") {
		t.Fatalf("ожидали ошибку про PASS3, получили: %v", err)
	}
}

func TestRefund_CreateRejected(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "OpStateExt") {
			fmt.Fprint(w, `<?xml version="1.0"?><OperationStateResponse xmlns="http://merchant.roboxchange.com/WebService/"><Result><Code>0</Code></Result><Info><OpKey>op-key</OpKey></Info></OperationStateResponse>`)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"success":false,"message":"AlreadyRefunded"}`)
	}))
	defer srv.Close()

	c := newTestClient(false)
	c.httpClient = srv.Client()
	c.opStateURL = srv.URL + "/OpStateExt"
	c.refundCreateURL = srv.URL + "/Refund/Create"

	err := c.Refund(context.Background(), "2000.00", 7)
	if err == nil || !strings.Contains(err.Error(), "AlreadyRefunded") {
		t.Fatalf("ожидали AlreadyRefunded, получили: %v", err)
	}
}

func TestBuildRefundJWT(t *testing.T) {
	token, err := buildRefundJWT("op-key-123", testPassword3)
	if err != nil {
		t.Fatal(err)
	}
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		t.Fatalf("ожидали 3 части JWT, получили %d", len(parts))
	}
}

// newTestRefundClient подключает httptest-сервер ко всем URL возврата.
func newTestRefundClient(srv *httptest.Server) *Client {
	c := newTestClient(false)
	c.httpClient = srv.Client()
	c.opStateURL = srv.URL + "/OpStateExt"
	c.refundCreateURL = srv.URL + "/Refund/Create"
	c.refundStateURL = srv.URL + "/Refund/GetState"
	return c
}

// --- helpers ---

func extractParam(rawURL, key string) string {
	// Простой парсер query-параметров для тестов
	parts := strings.SplitN(rawURL, "?", 2)
	if len(parts) < 2 {
		return ""
	}
	for _, pair := range strings.Split(parts[1], "&") {
		kv := strings.SplitN(pair, "=", 2)
		if len(kv) == 2 && kv[0] == key {
			return kv[1]
		}
	}
	return ""
}
