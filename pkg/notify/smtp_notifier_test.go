package notify

import (
	"bufio"
	"context"
	"net"
	"net/smtp"
	"strings"
	"testing"
)

// fakeSmtpServer поднимает простой SMTP-сервер в памяти, принимает одно соединение,
// возвращает 250 на каждую команду и собирает полученный DATA-блок.
func fakeSmtpServer(t *testing.T) (addr string, received chan string) {
	t.Helper()
	return fakeSmtpServerWith(t, nil)
}

// fakeSmtpServerWith — расширяемый вариант: overrides позволяет вернуть кастомный ответ
// для конкретного SMTP-префикса команды (например "AUTH" → "535 Auth failed").
// Если overrides == nil или ключ не найден, используется ответ по умолчанию.
func fakeSmtpServerWith(t *testing.T, overrides map[string]string) (addr string, received chan string) {
	t.Helper()
	received = make(chan string, 1)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("не удалось создать тест-сервер: %v", err)
	}

	reply := func(rw *bufio.ReadWriter, cmdUpper, defaultResp string) {
		resp := defaultResp
		// range по nil map — безопасный no-op, явная проверка не нужна.
		for prefix, r := range overrides {
			if strings.HasPrefix(cmdUpper, prefix) {
				resp = r
				break
			}
		}
		rw.WriteString(resp + "\r\n")
		rw.Flush()
	}

	go func() {
		defer ln.Close()
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()

		rw := bufio.NewReadWriter(bufio.NewReader(conn), bufio.NewWriter(conn))
		rw.WriteString("220 test.local ESMTP\r\n")
		rw.Flush()

		var dataLines strings.Builder
		inData := false

		for {
			line, err := rw.ReadString('\n')
			if err != nil {
				break
			}
			line = strings.TrimRight(line, "\r\n")

			if inData {
				if line == "." {
					rw.WriteString("250 OK\r\n")
					rw.Flush()
					received <- dataLines.String()
					break
				}
				dataLines.WriteString(line + "\n")
				continue
			}

			upper := strings.ToUpper(line)
			switch {
			case strings.HasPrefix(upper, "EHLO"), strings.HasPrefix(upper, "HELO"):
				// EHLO multiline — часть обязательного handshake, кастомизация не нужна
				rw.WriteString("250-test.local\r\n250 AUTH PLAIN LOGIN\r\n")
				rw.Flush()
			case strings.HasPrefix(upper, "AUTH"):
				reply(rw, upper, "235 Authentication successful")
			case strings.HasPrefix(upper, "MAIL FROM"):
				reply(rw, upper, "250 OK")
			case strings.HasPrefix(upper, "RCPT TO"):
				reply(rw, upper, "250 OK")
			case upper == "DATA":
				reply(rw, upper, "354 End data with <CR><LF>.<CR><LF>")
				inData = true
			case upper == "QUIT":
				rw.WriteString("221 Bye\r\n")
				rw.Flush()
				return
			default:
				rw.WriteString("250 OK\r\n")
				rw.Flush()
			}
		}
	}()

	return ln.Addr().String(), received
}

// closedAddr возвращает адрес, на котором уже никто не слушает (connection refused).
func closedAddr(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("Listen: %v", err)
	}
	addr := ln.Addr().String()
	ln.Close()
	return addr
}

func TestSmtpNotifier_NotifyOrderComplete(t *testing.T) {
	// Проверяем что пустой email — не ошибка.
	n := NewSmtpNotifier("localhost", 587, "u", "p", "from@test.com", "Test", "", "https://numaestra.ru")
	err := n.NotifyOrderComplete(context.Background(), OrderCompleteNotification{Email: ""})
	if err != nil {
		t.Fatalf("пустой email должен тихо пропускаться, got error: %v", err)
	}

	notif := OrderCompleteNotification{
		OrderID:     "test-order-123",
		Email:       "user@example.com",
		TrackURLs:   []string{"https://s3.example.com/track1.mp3", "https://s3.example.com/track2.mp3"},
		TracksCount: 2,
	}

	// Проверяем содержимое MIME-сообщения.
	msg := buildMIMEMessage("from@numaestra.ru", "Numaestra", notif.Email, "support@numaestra.ru", "Ваша песня готова — Numaestra", "plain", "<html>body</html>")
	if !strings.Contains(msg, "From:") {
		t.Errorf("ожидался заголовок From, получили:\n%s", msg)
	}
	if !strings.Contains(msg, "from@numaestra.ru") {
		t.Errorf("ожидался адрес отправителя, получили:\n%s", msg)
	}
	if !strings.Contains(msg, "To: user@example.com") {
		t.Errorf("ожидался заголовок To, получили:\n%s", msg)
	}
	if !strings.Contains(msg, "Reply-To: support@numaestra.ru") {
		t.Errorf("ожидался Reply-To, получили:\n%s", msg)
	}
	if !strings.Contains(msg, "multipart/alternative") {
		t.Errorf("ожидался multipart/alternative, получили:\n%s", msg)
	}
	if !strings.Contains(msg, "text/plain") || !strings.Contains(msg, "text/html") {
		t.Errorf("ожидались text/plain и text/html, получили:\n%s", msg)
	}

	// Проверяем HTML-тело письма.
	body := n.buildBody(notif)
	if !strings.Contains(body, "test-ord") {
		t.Error("тело письма должно содержать короткий OrderID")
	}
	if !strings.Contains(body, "https://numaestra.ru/status?order_id=test-order-123") {
		t.Error("тело письма должно содержать абсолютную ссылку на статус заказа")
	}
	if !strings.Contains(body, "track1.mp3") {
		t.Error("тело письма должно содержать ссылку на трек")
	}
	if !strings.Contains(body, "Вариант 1") {
		t.Error("тело письма должно содержать подпись трека")
	}
}

// --- sendSTARTTLS ---

func TestSendSTARTTLS_DialFails(t *testing.T) {
	addr := closedAddr(t)
	n := NewSmtpNotifier("testhost", 587, "u", "p", "from@test.com", "Test", "", "https://numaestra.ru")
	err := n.sendSTARTTLS(addr, smtp.PlainAuth("", "u", "p", "testhost"), "to@test.com", "msg")
	if err == nil {
		t.Fatal("ожидали ошибку при подключении к закрытому серверу")
	}
	if !strings.Contains(err.Error(), "smtp dial") {
		t.Errorf("ошибка должна содержать 'smtp dial', получили: %v", err)
	}
}

func TestSendSTARTTLS_StartTLSFails(t *testing.T) {
	// Plain-сервер не поддерживает STARTTLS — клиент получит 250 вместо ожидаемых 220.
	addr, _ := fakeSmtpServer(t)
	n := NewSmtpNotifier("testhost", 587, "u", "p", "from@test.com", "Test", "", "https://numaestra.ru")
	err := n.sendSTARTTLS(addr, smtp.PlainAuth("", "u", "p", "testhost"), "to@test.com", "msg")
	if err == nil {
		t.Fatal("ожидали ошибку STARTTLS при подключении к plain-серверу")
	}
	if !strings.Contains(err.Error(), "smtp starttls") {
		t.Errorf("ошибка должна содержать 'smtp starttls', получили: %v", err)
	}
}

// --- sendImplicitTLS ---

func TestSendImplicitTLS_DialFails(t *testing.T) {
	addr := closedAddr(t)
	n := NewSmtpNotifier("testhost", 465, "u", "p", "from@test.com", "Test", "", "https://numaestra.ru")
	err := n.sendImplicitTLS(addr, smtp.PlainAuth("", "u", "p", "testhost"), "to@test.com", "msg")
	if err == nil {
		t.Fatal("ожидали ошибку при подключении к закрытому серверу")
	}
	if !strings.Contains(err.Error(), "smtp tls dial") {
		t.Errorf("ошибка должна содержать 'smtp tls dial', получили: %v", err)
	}
}

// --- deliverMessage ---

func TestDeliverMessage_Success(t *testing.T) {
	addr, received := fakeSmtpServer(t)
	host, _, _ := net.SplitHostPort(addr)

	client, err := smtp.Dial(addr)
	if err != nil {
		t.Fatalf("smtp.Dial: %v", err)
	}
	defer client.Quit() //nolint:errcheck

	n := NewSmtpNotifier(host, 587, "user@test.com", "pass", "from@test.com", "Test", "", "https://numaestra.ru")
	auth := smtp.PlainAuth("", "user@test.com", "pass", host)
	if err := n.deliverMessage(client, auth, "to@test.com", "Subject: hi\r\n\r\nhello world"); err != nil {
		t.Fatalf("deliverMessage: %v", err)
	}

	body := <-received
	if !strings.Contains(body, "hello world") {
		t.Errorf("fake server не получил тело сообщения, got: %q", body)
	}
}

func TestDeliverMessage_AuthFails(t *testing.T) {
	addr, _ := fakeSmtpServerWith(t, map[string]string{"AUTH": "535 Authentication credentials invalid"})
	host, _, _ := net.SplitHostPort(addr)

	client, err := smtp.Dial(addr)
	if err != nil {
		t.Fatalf("smtp.Dial: %v", err)
	}
	defer client.Quit() //nolint:errcheck

	n := NewSmtpNotifier(host, 587, "bad", "creds", "from@test.com", "Test", "", "https://numaestra.ru")
	auth := smtp.PlainAuth("", "bad", "creds", host)
	err = n.deliverMessage(client, auth, "to@test.com", "msg")
	if err == nil {
		t.Fatal("ожидали ошибку при отказе аутентификации")
	}
	if !strings.Contains(err.Error(), "smtp auth") {
		t.Errorf("ошибка должна содержать 'smtp auth', получили: %v", err)
	}
}

func TestDeliverMessage_RcptFails(t *testing.T) {
	addr, _ := fakeSmtpServerWith(t, map[string]string{"RCPT TO": "550 No such user"})
	host, _, _ := net.SplitHostPort(addr)

	client, err := smtp.Dial(addr)
	if err != nil {
		t.Fatalf("smtp.Dial: %v", err)
	}
	defer client.Quit() //nolint:errcheck

	n := NewSmtpNotifier(host, 587, "u", "p", "from@test.com", "Test", "", "https://numaestra.ru")
	auth := smtp.PlainAuth("", "u", "p", host)
	err = n.deliverMessage(client, auth, "unknown@test.com", "msg")
	if err == nil {
		t.Fatal("ожидали ошибку при отказе RCPT TO")
	}
	if !strings.Contains(err.Error(), "smtp rcpt") {
		t.Errorf("ошибка должна содержать 'smtp rcpt', получили: %v", err)
	}
}

// --- NotifyOrderComplete ---

// TestNotifyOrderComplete_STARTTLS_Success проверяет полный путь уведомления через порт 587.
// Используем dialPlain-инъекцию, чтобы подключиться к plain fake-серверу без TLS-handshake.
func TestNotifyOrderComplete_STARTTLS_Success(t *testing.T) {
	addr, received := fakeSmtpServer(t)
	host, _, _ := net.SplitHostPort(addr)

	n := NewSmtpNotifier(host, 587, "user@test.com", "pass", "from@test.com", "Test", "", "https://numaestra.ru")
	// Игнорируем сконструированный addr (127.0.0.1:587) и подключаемся к fake-серверу.
	n.dialPlain = func(_ string) (*smtp.Client, error) { return smtp.Dial(addr) }

	err := n.NotifyOrderComplete(context.Background(), OrderCompleteNotification{
		OrderID:     "order-abc",
		Email:       "customer@example.com",
		TrackURLs:   []string{"https://cdn.example.com/track1.mp3"},
		TracksCount: 1,
	})
	if err != nil {
		t.Fatalf("NotifyOrderComplete: %v", err)
	}

	body := <-received
	if !strings.Contains(body, "order-abc") {
		t.Errorf("письмо должно содержать OrderID, got: %q", body)
	}
}

// TestNotifyOrderComplete_STARTTLS_DialFails проверяет обработку ошибки соединения на порту 587.
func TestNotifyOrderComplete_STARTTLS_DialFails(t *testing.T) {
	addr := closedAddr(t)
	host, _, _ := net.SplitHostPort(addr)

	n := NewSmtpNotifier(host, 587, "u", "p", "from@test.com", "Test", "", "https://numaestra.ru")
	err := n.NotifyOrderComplete(context.Background(), OrderCompleteNotification{
		Email: "customer@example.com",
	})
	if err == nil {
		t.Fatal("ожидали ошибку при недоступном сервере")
	}
}

// TestNotifyOrderComplete_ImplicitTLS_DialFails проверяет обработку ошибки соединения на порту 465.
func TestNotifyOrderComplete_ImplicitTLS_DialFails(t *testing.T) {
	addr := closedAddr(t)
	host, _, _ := net.SplitHostPort(addr)

	n := NewSmtpNotifier(host, 465, "u", "p", "from@test.com", "Test", "", "https://numaestra.ru")
	err := n.NotifyOrderComplete(context.Background(), OrderCompleteNotification{
		Email: "customer@example.com",
	})
	if err == nil {
		t.Fatal("ожидали ошибку при недоступном TLS-сервере")
	}
}
