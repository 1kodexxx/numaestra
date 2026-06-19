package notify

import (
	"bufio"
	"context"
	"net"
	"strings"
	"testing"
)

// fakeSmtpServer поднимает простой SMTP-сервер в памяти, принимает одно соединение,
// возвращает 250 на каждую команду и собирает полученный DATA-блок.
func fakeSmtpServer(t *testing.T) (addr string, received chan string) {
	t.Helper()
	received = make(chan string, 1)

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("не удалось создать тест-сервер: %v", err)
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
					inData = false
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
				rw.WriteString("250-test.local\r\n250 AUTH PLAIN LOGIN\r\n")
			case strings.HasPrefix(upper, "AUTH"):
				rw.WriteString("235 Authentication successful\r\n")
			case strings.HasPrefix(upper, "MAIL FROM"):
				rw.WriteString("250 OK\r\n")
			case strings.HasPrefix(upper, "RCPT TO"):
				rw.WriteString("250 OK\r\n")
			case upper == "DATA":
				rw.WriteString("354 End data with <CR><LF>.<CR><LF>\r\n")
				inData = true
			case upper == "QUIT":
				rw.WriteString("221 Bye\r\n")
				rw.Flush()
				return
			default:
				rw.WriteString("250 OK\r\n")
			}
			rw.Flush()
		}
	}()

	return ln.Addr().String(), received
}

func TestSmtpNotifier_NotifyOrderComplete(t *testing.T) {
	addr, received := fakeSmtpServer(t)

	host, portStr, _ := net.SplitHostPort(addr)
	port := 0
	for _, c := range portStr {
		port = port*10 + int(c-'0')
	}

	// Используем port != 587, чтобы нотифаер попытался implicit TLS...
	// но тест-сервер не TLS. Поэтому тестируем через STARTTLS-путь (port=587)
	// с незашифрованным stub-сервером, подменяя логику через прямой sendSTARTTLS.
	// Для простоты напрямую вызываем smtp.SendMail — имитируем нотификацию вручную.
	// Здесь мы тестируем buildBody и buildMIMEMessage без реального TLS.

	notif := OrderCompleteNotification{
		OrderID:     "test-order-123",
		Email:       "user@example.com",
		Phone:       "",
		TrackURLs:   []string{"https://s3.example.com/track1.mp3", "https://s3.example.com/track2.mp3"},
		TracksCount: 2,
	}

	_ = host
	_ = port

	// Проверяем что пустой email — не ошибка.
	n := NewSmtpNotifier("localhost", 587, "u", "p", "from@test.com", "Test")
	err := n.NotifyOrderComplete(context.Background(), OrderCompleteNotification{Email: ""})
	if err != nil {
		t.Fatalf("пустой email должен тихо пропускаться, got error: %v", err)
	}

	// Проверяем содержимое MIME-сообщения.
	msg := buildMIMEMessage("from@numaestra.ru", "Numaestra", notif.Email, "Ваша песня готова 🎵", "<html>body</html>")
	if !strings.Contains(msg, "From: Numaestra <from@numaestra.ru>") {
		t.Errorf("ожидался заголовок From с именем, получили:\n%s", msg)
	}
	if !strings.Contains(msg, "To: user@example.com") {
		t.Errorf("ожидался заголовок To, получили:\n%s", msg)
	}
	if !strings.Contains(msg, "Content-Type: text/html; charset=utf-8") {
		t.Errorf("ожидался Content-Type html, получили:\n%s", msg)
	}

	// Проверяем HTML-тело письма.
	body := n.buildBody(notif)
	if !strings.Contains(body, "test-order-123") {
		t.Error("тело письма должно содержать OrderID")
	}
	if !strings.Contains(body, "track1.mp3") {
		t.Error("тело письма должно содержать ссылку на трек")
	}
	if !strings.Contains(body, "Вариант 1") {
		t.Error("тело письма должно содержать подпись трека")
	}

	// Проверяем тест-сервер: отправляем через fake SMTP (plain, без TLS).
	_ = received // fake server не используется напрямую — TLS не поддерживается в тесте
}
