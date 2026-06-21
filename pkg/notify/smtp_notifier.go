package notify

import (
	"context"
	"crypto/tls"
	"fmt"
	"html"
	"net/smtp"
	"strings"
	"time"
)

// SmtpNotifier отправляет HTML-письмо о готовности заказа через стандартный SMTP.
// Поддерживает Mailgun, Yandex 360, SendGrid и любой другой провайдер с SMTP-доступом.
// Соединение устанавливается по TLS на порту 465 (implicit TLS) или через STARTTLS на 587.
// Для порта 587 используется STARTTLS; для любого другого порта — implicit TLS (tls.Dial).
type SmtpNotifier struct {
	host     string
	port     int
	user     string
	password string
	from     string
	fromName string

	// dialPlain заменяет smtp.Dial+StartTLS в sendSTARTTLS — используется только в тестах
	// (подключение к plain fake-серверу без TLS-handshake).
	dialPlain func(addr string) (*smtp.Client, error)
}

// NewSmtpNotifier создаёт SMTP-отправщик уведомлений.
func NewSmtpNotifier(host string, port int, user, password, from, fromName string) *SmtpNotifier {
	return &SmtpNotifier{
		host:     host,
		port:     port,
		user:     user,
		password: password,
		from:     from,
		fromName: fromName,
	}
}

func (n *SmtpNotifier) NotifyOrderComplete(_ context.Context, notif OrderCompleteNotification) error {
	if notif.Email == "" {
		return nil // email необязателен — молча пропускаем
	}

	subject := "Ваша песня готова 🎵"
	body := n.buildBody(notif)
	return n.send(notif.Email, subject, body)
}

func (n *SmtpNotifier) NotifyAdminFeedback(_ context.Context, notif AdminFeedbackNotification) error {
	if notif.Email == "" {
		return nil // email необязателен — молча пропускаем
	}

	subject := "Сообщение по вашему заказу — Numaestra"
	body := n.buildFeedbackBody(notif)
	return n.send(notif.Email, subject, body)
}

// send отправляет письмо с заданной темой и HTML-телом, выбирая STARTTLS (587)
// либо implicit TLS (любой другой порт).
func (n *SmtpNotifier) send(to, subject, htmlBody string) error {
	msg := buildMIMEMessage(n.from, n.fromName, to, subject, htmlBody)

	addr := fmt.Sprintf("%s:%d", n.host, n.port)
	auth := smtp.PlainAuth("", n.user, n.password, n.host)

	if n.port == 587 {
		return n.sendSTARTTLS(addr, auth, to, msg)
	}
	return n.sendImplicitTLS(addr, auth, to, msg)
}

// sendSTARTTLS устанавливает соединение по STARTTLS (порт 587).
func (n *SmtpNotifier) sendSTARTTLS(addr string, auth smtp.Auth, to, msg string) error {
	var client *smtp.Client
	var err error

	if n.dialPlain != nil {
		// Тестовый путь: получаем готовый клиент без TLS-апгрейда.
		client, err = n.dialPlain(addr)
		if err != nil {
			return fmt.Errorf("smtp dial: %w", err)
		}
	} else {
		client, err = smtp.Dial(addr)
		if err != nil {
			return fmt.Errorf("smtp dial: %w", err)
		}
		tlsCfg := &tls.Config{ServerName: n.host, MinVersion: tls.VersionTLS12}
		if err = client.StartTLS(tlsCfg); err != nil {
			return fmt.Errorf("smtp starttls: %w", err)
		}
	}
	defer client.Quit() //nolint:errcheck
	return n.deliverMessage(client, auth, to, msg)
}

// sendImplicitTLS устанавливает соединение с неявным TLS (порт 465).
func (n *SmtpNotifier) sendImplicitTLS(addr string, auth smtp.Auth, to, msg string) error {
	tlsCfg := &tls.Config{ServerName: n.host, MinVersion: tls.VersionTLS12}
	conn, err := tls.Dial("tcp", addr, tlsCfg)
	if err != nil {
		return fmt.Errorf("smtp tls dial: %w", err)
	}
	defer conn.Close()

	client, err := smtp.NewClient(conn, n.host)
	if err != nil {
		return fmt.Errorf("smtp new client: %w", err)
	}
	defer client.Quit() //nolint:errcheck

	return n.deliverMessage(client, auth, to, msg)
}

func (n *SmtpNotifier) deliverMessage(client *smtp.Client, auth smtp.Auth, to, msg string) error {
	if err := client.Auth(auth); err != nil {
		return fmt.Errorf("smtp auth: %w", err)
	}
	if err := client.Mail(n.from); err != nil {
		return fmt.Errorf("smtp mail from: %w", err)
	}
	if err := client.Rcpt(to); err != nil {
		return fmt.Errorf("smtp rcpt: %w", err)
	}
	wc, err := client.Data()
	if err != nil {
		return fmt.Errorf("smtp data: %w", err)
	}
	defer wc.Close()
	if _, err = fmt.Fprint(wc, msg); err != nil {
		return fmt.Errorf("smtp write body: %w", err)
	}
	return nil
}

// buildBody формирует HTML-тело письма.
func (n *SmtpNotifier) buildBody(notif OrderCompleteNotification) string {
	var tracks strings.Builder
	for i, url := range notif.TrackURLs {
		tracks.WriteString(fmt.Sprintf(
			`<li style="margin:8px 0"><a href="%s" style="color:#a855f7;font-weight:600">🎧 Вариант %d</a></li>`,
			url, i+1,
		))
	}

	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="ru">
<head><meta charset="utf-8"><title>Ваша песня готова</title></head>
<body style="background:#0f0f1a;color:#e2e8f0;font-family:'Segoe UI',Arial,sans-serif;margin:0;padding:0">
  <table width="100%%" cellpadding="0" cellspacing="0">
    <tr><td align="center" style="padding:40px 16px">
      <table width="600" cellpadding="0" cellspacing="0" style="background:#1a1a2e;border-radius:16px;overflow:hidden;max-width:600px">
        <tr>
          <td style="background:linear-gradient(135deg,#7c3aed,#a855f7);padding:40px;text-align:center">
            <div style="font-size:48px">🎵</div>
            <h1 style="margin:16px 0 0;color:#fff;font-size:28px;font-weight:700">Ваша песня готова!</h1>
            <p style="margin:8px 0 0;color:#e9d5ff;font-size:16px">Заказ #%s успешно выполнен</p>
          </td>
        </tr>
        <tr>
          <td style="padding:40px">
            <p style="font-size:16px;line-height:1.6;color:#cbd5e1;margin:0 0 24px">
              Мы с радостью сообщаем, что ваша персональная песня готова. 
              Все треки доступны по ссылкам ниже.
            </p>
            <ul style="list-style:none;padding:0;margin:0 0 32px">
              %s
            </ul>
            <div style="text-align:center">
              <a href="%s" style="display:inline-block;background:linear-gradient(135deg,#7c3aed,#a855f7);color:#fff;text-decoration:none;padding:14px 32px;border-radius:8px;font-size:16px;font-weight:600">
                Слушать все треки
              </a>
            </div>
            <p style="font-size:13px;color:#64748b;margin:32px 0 0;text-align:center">
              Ссылки действуют бессрочно. Сохраните письмо, чтобы не потерять доступ.<br>
              Дата: %s
            </p>
          </td>
        </tr>
        <tr>
          <td style="background:#111827;padding:20px;text-align:center">
            <p style="margin:0;font-size:13px;color:#475569">
              © %d Numaestra. Персональные песни на заказ.
            </p>
          </td>
        </tr>
      </table>
    </td></tr>
  </table>
</body>
</html>`,
		notif.OrderID,
		tracks.String(),
		orderStatusURL(notif.OrderID, notif.AccessToken),
		time.Now().Format("02.01.2006"),
		time.Now().Year(),
	)
}

// buildFeedbackBody формирует HTML-тело письма с обратной связью администратора.
// Сообщение администратора экранируется (html.EscapeString) перед вставкой в
// HTML — это произвольный текст, введённый человеком через админку, и без
// экранирования он мог бы исполниться как HTML/JS в почтовом клиенте получателя.
func (n *SmtpNotifier) buildFeedbackBody(notif AdminFeedbackNotification) string {
	escaped := strings.ReplaceAll(html.EscapeString(notif.Message), "\n", "<br>")

	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="ru">
<head><meta charset="utf-8"><title>Сообщение по вашему заказу</title></head>
<body style="background:#0f0f1a;color:#e2e8f0;font-family:'Segoe UI',Arial,sans-serif;margin:0;padding:0">
  <table width="100%%" cellpadding="0" cellspacing="0">
    <tr><td align="center" style="padding:40px 16px">
      <table width="600" cellpadding="0" cellspacing="0" style="background:#1a1a2e;border-radius:16px;overflow:hidden;max-width:600px">
        <tr>
          <td style="background:linear-gradient(135deg,#7c3aed,#a855f7);padding:32px;text-align:center">
            <div style="font-size:40px">💬</div>
            <h1 style="margin:12px 0 0;color:#fff;font-size:22px;font-weight:700">Сообщение по заказу</h1>
            <p style="margin:8px 0 0;color:#e9d5ff;font-size:14px">Заказ #%s</p>
          </td>
        </tr>
        <tr>
          <td style="padding:40px">
            <p style="font-size:16px;line-height:1.6;color:#cbd5e1;margin:0">
              %s
            </p>
          </td>
        </tr>
        <tr>
          <td style="background:#111827;padding:20px;text-align:center">
            <p style="margin:0;font-size:13px;color:#475569">
              © %d Numaestra. Персональные песни на заказ.
            </p>
          </td>
        </tr>
      </table>
    </td></tr>
  </table>
</body>
</html>`,
		notif.OrderID,
		escaped,
		time.Now().Year(),
	)
}

// orderStatusURL формирует ссылку на страницу статуса заказа.
// Если передан accessToken — включаем его как query-параметр, чтобы
// страница могла автоматически подставить токен без ввода вручную.
func orderStatusURL(orderID, accessToken string) string {
	if accessToken != "" {
		return fmt.Sprintf("/status?order_id=%s&token=%s", orderID, accessToken)
	}
	return fmt.Sprintf("/status?order_id=%s", orderID)
}

// buildMIMEMessage формирует MIME-сообщение с заголовками RFC 5322.
func buildMIMEMessage(from, fromName, to, subject, htmlBody string) string {
	displayFrom := from
	if fromName != "" {
		displayFrom = fmt.Sprintf("%s <%s>", fromName, from)
	}

	var sb strings.Builder
	sb.WriteString("From: " + displayFrom + "\r\n")
	sb.WriteString("To: " + to + "\r\n")
	sb.WriteString("Subject: " + subject + "\r\n")
	sb.WriteString("MIME-Version: 1.0\r\n")
	sb.WriteString("Content-Type: text/html; charset=utf-8\r\n")
	sb.WriteString("\r\n")
	sb.WriteString(htmlBody)
	return sb.String()
}
