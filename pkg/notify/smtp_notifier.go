package notify

import (
	"context"
	"crypto/tls"
	"fmt"
	"html"
	"net/smtp"
	"net/url"
	"strings"
	"time"
)

// SmtpNotifier отправляет HTML-письмо о готовности заказа через стандартный SMTP.
// Поддерживает Mailgun, Yandex 360, SendGrid, RuSender и любой другой провайдер с SMTP-доступом.
// Соединение устанавливается по TLS на порту 465 (implicit TLS) или через STARTTLS на 587.
type SmtpNotifier struct {
	host          string
	port          int
	user          string
	password      string
	from          string
	fromName      string
	publicAppURL  string // абсолютный URL сайта для ссылок в письмах
	dialPlain     func(addr string) (*smtp.Client, error)
}

// NewSmtpNotifier создаёт SMTP-отправщик уведомлений.
// publicAppURL — публичный адрес сайта без завершающего слэша (https://numaestra.ru).
func NewSmtpNotifier(host string, port int, user, password, from, fromName, publicAppURL string) *SmtpNotifier {
	return &SmtpNotifier{
		host:         host,
		port:         port,
		user:         user,
		password:     password,
		from:         from,
		fromName:     fromName,
		publicAppURL: strings.TrimRight(publicAppURL, "/"),
	}
}

func (n *SmtpNotifier) NotifyOrderComplete(_ context.Context, notif OrderCompleteNotification) error {
	if notif.Email == "" {
		return nil
	}

	subject := "Ваша песня готова — Numaestra"
	body := n.buildBody(notif)
	return n.send(notif.Email, subject, body)
}

func (n *SmtpNotifier) NotifyAdminFeedback(_ context.Context, notif AdminFeedbackNotification) error {
	if notif.Email == "" {
		return nil
	}

	subject := "Сообщение по вашему заказу — Numaestra"
	body := n.buildFeedbackBody(notif)
	return n.send(notif.Email, subject, body)
}

func (n *SmtpNotifier) send(to, subject, htmlBody string) error {
	msg := buildMIMEMessage(n.from, n.fromName, to, subject, htmlBody)

	addr := fmt.Sprintf("%s:%d", n.host, n.port)
	auth := smtp.PlainAuth("", n.user, n.password, n.host)

	if n.port == 587 {
		return n.sendSTARTTLS(addr, auth, to, msg)
	}
	return n.sendImplicitTLS(addr, auth, to, msg)
}

func (n *SmtpNotifier) sendSTARTTLS(addr string, auth smtp.Auth, to, msg string) error {
	var client *smtp.Client
	var err error

	if n.dialPlain != nil {
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

// buildBody формирует HTML-тело письма в фирменном стиле Numaestra.
func (n *SmtpNotifier) buildBody(notif OrderCompleteNotification) string {
	statusURL := n.orderStatusURL(notif.OrderID, notif.AccessToken)
	escapedStatusURL := html.EscapeString(statusURL)
	shortOrderID := notif.OrderID
	if len(shortOrderID) > 8 {
		shortOrderID = shortOrderID[:8]
	}

	var tracks strings.Builder
	for i, trackURL := range notif.TrackURLs {
		escaped := html.EscapeString(trackURL)
		fmt.Fprintf(&tracks,
			`<tr>
			  <td style="padding:10px 0;border-bottom:1px solid rgba(255,255,255,0.06)">
			    <a href="%s" style="color:#00e5c0;text-decoration:none;font-size:15px;font-weight:600">
			      Вариант %d →
			    </a>
			  </td>
			</tr>`,
			escaped, i+1,
		)
	}

	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="ru">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>Ваша песня готова — Numaestra</title>
</head>
<body style="margin:0;padding:0;background:#080808;color:#ffffff;font-family:'Segoe UI',Arial,Helvetica,sans-serif">
  <table role="presentation" width="100%%" cellpadding="0" cellspacing="0" style="background:#080808">
    <tr>
      <td align="center" style="padding:40px 16px">
        <table role="presentation" width="600" cellpadding="0" cellspacing="0" style="max-width:600px;width:100%%;background:#0f0f0f;border:1px solid rgba(255,255,255,0.07);border-radius:20px;overflow:hidden">
          <tr>
            <td style="padding:36px 32px 28px;text-align:center;background:radial-gradient(ellipse 80%% 60%% at 50%% 0%%, rgba(0,229,192,0.14) 0%%, transparent 70%%)">
              <div style="font-size:13px;font-weight:700;letter-spacing:0.12em;text-transform:uppercase;color:#00e5c0;margin-bottom:12px">Numaestra</div>
              <div style="font-size:40px;line-height:1;margin-bottom:16px">🎵</div>
              <h1 style="margin:0 0 10px;font-size:28px;font-weight:800;letter-spacing:-0.03em;color:#ffffff">Ваша песня готова!</h1>
              <p style="margin:0;font-size:15px;line-height:1.5;color:rgba(255,255,255,0.55)">
                %d версии трека ждут вас на сайте
              </p>
            </td>
          </tr>
          <tr>
            <td style="padding:8px 32px 0">
              <table role="presentation" width="100%%" cellpadding="0" cellspacing="0">
                <tr>
                  <td style="padding:14px 16px;background:rgba(0,229,192,0.06);border:1px solid rgba(0,229,192,0.16);border-radius:12px">
                    <div style="font-size:12px;color:rgba(255,255,255,0.45);margin-bottom:4px">Заказ</div>
                    <div style="font-size:14px;font-weight:600;color:#ffffff;font-family:ui-monospace,Consolas,monospace">#%s</div>
                  </td>
                </tr>
              </table>
            </td>
          </tr>
          <tr>
            <td style="padding:28px 32px 8px">
              <p style="margin:0 0 20px;font-size:15px;line-height:1.65;color:rgba(255,255,255,0.72)">
                Мы закончили генерацию вашей персональной песни. Все версии доступны в плеере на сайте — ссылки действуют бессрочно.
              </p>
              <table role="presentation" width="100%%" cellpadding="0" cellspacing="0" style="margin-bottom:28px">
                %s
              </table>
              <table role="presentation" width="100%%" cellpadding="0" cellspacing="0">
                <tr>
                  <td align="center">
                    <a href="%s" style="display:inline-block;background:linear-gradient(135deg,#00e5c0,#00bfa5);color:#080808;text-decoration:none;padding:15px 36px;border-radius:12px;font-size:16px;font-weight:700;letter-spacing:-0.01em">
                      Слушать все версии
                    </a>
                  </td>
                </tr>
              </table>
              <p style="margin:24px 0 0;font-size:12px;line-height:1.6;color:rgba(255,255,255,0.35);text-align:center">
                Если кнопка не открывается, скопируйте ссылку:<br>
                <a href="%s" style="color:#00e5c0;word-break:break-all">%s</a>
              </p>
            </td>
          </tr>
          <tr>
            <td style="padding:20px 32px 28px;border-top:1px solid rgba(255,255,255,0.06);text-align:center">
              <p style="margin:0 0 8px;font-size:12px;color:rgba(255,255,255,0.35)">
                4 версии · готово за ~10 минут · без подписок
              </p>
              <p style="margin:0;font-size:12px;color:rgba(255,255,255,0.28)">
                © %d Numaestra · Персональные песни на заказ
              </p>
            </td>
          </tr>
        </table>
      </td>
    </tr>
  </table>
</body>
</html>`,
		notif.TracksCount,
		html.EscapeString(shortOrderID),
		tracks.String(),
		escapedStatusURL,
		escapedStatusURL,
		escapedStatusURL,
		time.Now().Year(),
	)
}

func (n *SmtpNotifier) buildFeedbackBody(notif AdminFeedbackNotification) string {
	escaped := strings.ReplaceAll(html.EscapeString(notif.Message), "\n", "<br>")
	shortOrderID := html.EscapeString(notif.OrderID)
	if len(notif.OrderID) > 8 {
		shortOrderID = html.EscapeString(notif.OrderID[:8])
	}

	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="ru">
<head><meta charset="utf-8"><title>Сообщение по заказу — Numaestra</title></head>
<body style="margin:0;padding:0;background:#080808;color:#ffffff;font-family:'Segoe UI',Arial,Helvetica,sans-serif">
  <table role="presentation" width="100%%" cellpadding="0" cellspacing="0">
    <tr><td align="center" style="padding:40px 16px">
      <table role="presentation" width="600" cellpadding="0" cellspacing="0" style="max-width:600px;width:100%%;background:#0f0f0f;border:1px solid rgba(255,255,255,0.07);border-radius:20px;overflow:hidden">
        <tr>
          <td style="padding:32px;text-align:center;background:radial-gradient(ellipse 80%% 60%% at 50%% 0%%, rgba(0,229,192,0.12) 0%%, transparent 70%%)">
            <div style="font-size:13px;font-weight:700;letter-spacing:0.12em;text-transform:uppercase;color:#00e5c0;margin-bottom:10px">Numaestra</div>
            <div style="font-size:36px;margin-bottom:12px">💬</div>
            <h1 style="margin:0;font-size:22px;font-weight:800;color:#ffffff">Сообщение по заказу</h1>
            <p style="margin:10px 0 0;font-size:14px;color:rgba(255,255,255,0.5)">#%s</p>
          </td>
        </tr>
        <tr>
          <td style="padding:32px">
            <p style="margin:0;font-size:16px;line-height:1.65;color:rgba(255,255,255,0.78)">%s</p>
          </td>
        </tr>
        <tr>
          <td style="padding:20px 32px;border-top:1px solid rgba(255,255,255,0.06);text-align:center">
            <p style="margin:0;font-size:12px;color:rgba(255,255,255,0.28)">© %d Numaestra</p>
          </td>
        </tr>
      </table>
    </td></tr>
  </table>
</body>
</html>`,
		shortOrderID,
		escaped,
		time.Now().Year(),
	)
}

// orderStatusURL формирует абсолютную ссылку на страницу статуса заказа.
// Относительные /status?... ломаются на click-tracking RuSender (редирект на
// clicks.senderclick.net/status → 404).
func (n *SmtpNotifier) orderStatusURL(orderID, accessToken string) string {
	q := url.Values{}
	q.Set("order_id", orderID)
	if accessToken != "" {
		q.Set("token", accessToken)
	}
	path := "/status?" + q.Encode()
	if n.publicAppURL == "" {
		return path
	}
	return n.publicAppURL + path
}

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
