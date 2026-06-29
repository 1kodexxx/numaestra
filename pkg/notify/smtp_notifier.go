package notify

import (
	"context"
	"crypto/tls"
	"fmt"
	"html"
	"net/smtp"
	"net/url"
	"strings"
)

// SmtpNotifier отправляет HTML-письмо о готовности заказа через стандартный SMTP.
// Поддерживает Mailgun, Yandex 360, SendGrid, RuSender и любой другой провайдер с SMTP-доступом.
// Соединение устанавливается по TLS на порту 465 (implicit TLS) или через STARTTLS на 587.
type SmtpNotifier struct {
	host         string
	port         int
	user         string
	password     string
	from         string
	fromName     string
	replyTo      string
	publicAppURL string // абсолютный URL сайта для ссылок в письмах
	adminEmail   string // ADMIN_NOTIFY_EMAIL — куда слать админские события (пусто → не слать)
	dialPlain    func(addr string) (*smtp.Client, error)
}

// NewSmtpNotifier создаёт SMTP-отправщик уведомлений.
// publicAppURL — публичный адрес сайта без завершающего слэша (https://numaestra.ru).
// replyTo — адрес для ответов клиента (пустой → support@домен отправителя).
func NewSmtpNotifier(host string, port int, user, password, from, fromName, replyTo, publicAppURL string) *SmtpNotifier {
	rt := strings.TrimSpace(replyTo)
	if rt == "" {
		rt = defaultReplyTo(from)
	}
	return &SmtpNotifier{
		host:         host,
		port:         port,
		user:         user,
		password:     password,
		from:         from,
		fromName:     fromName,
		replyTo:      rt,
		publicAppURL: strings.TrimRight(publicAppURL, "/"),
	}
}

// WithAdminEmail включает админские уведомления (новая оплата, демо, провал
// генерации) на указанный адрес. Пустая строка оставляет их выключенными.
func (n *SmtpNotifier) WithAdminEmail(email string) *SmtpNotifier {
	n.adminEmail = strings.TrimSpace(email)
	return n
}

func (n *SmtpNotifier) NotifyOrderComplete(_ context.Context, notif OrderCompleteNotification) error {
	if notif.Email == "" {
		return nil
	}

	subject := "Ваша песня готова — Numaestra"
	htmlBody := n.buildBody(notif)
	textBody := n.buildPlainBody(notif)
	return n.send(notif.Email, subject, textBody, htmlBody)
}

func (n *SmtpNotifier) NotifyOrderFailed(_ context.Context, notif OrderFailedNotification) error {
	if notif.Email == "" {
		return nil
	}
	subject := "Ошибка генерации песни — Numaestra"
	htmlBody := n.buildFailedBody(notif)
	textBody := n.buildPlainFailedBody(notif)
	return n.send(notif.Email, subject, textBody, htmlBody)
}

func (n *SmtpNotifier) NotifyAdminFeedback(_ context.Context, notif AdminFeedbackNotification) error {
	if notif.Email == "" {
		return nil
	}

	subject := "Сообщение по вашему заказу — Numaestra"
	htmlBody := n.buildFeedbackBody(notif)
	textBody := n.buildPlainFeedbackBody(notif)
	return n.send(notif.Email, subject, textBody, htmlBody)
}

func (n *SmtpNotifier) NotifyAccessLink(_ context.Context, notif AccessLinkNotification) error {
	if notif.Email == "" {
		return nil
	}

	subject := "Ссылка на ваш заказ — Numaestra"
	htmlBody := n.buildAccessLinkBody(notif)
	textBody := n.buildPlainAccessLinkBody(notif)
	return n.send(notif.Email, subject, textBody, htmlBody)
}

// NotifyAdmin шлёт письмо администратору о событии заказа на ADMIN_NOTIFY_EMAIL.
// Без настроенного адреса — молча выходит (не ошибка).
func (n *SmtpNotifier) NotifyAdmin(_ context.Context, notif AdminEventNotification) error {
	if n.adminEmail == "" {
		return nil
	}
	subject, htmlBody, textBody := n.buildAdminEmail(notif)
	return n.send(n.adminEmail, subject, textBody, htmlBody)
}

// buildAdminEmail подбирает тему и тело письма под тип события.
func (n *SmtpNotifier) buildAdminEmail(notif AdminEventNotification) (subject, htmlBody, textBody string) {
	short := shortOrderID(notif.OrderID)
	adminURL := n.adminOrderURL(notif.OrderID)

	var emoji, title, lead string
	switch notif.Kind {
	case AdminEventPaidOrder:
		subject = fmt.Sprintf("💰 Новая оплата · заказ #%d — Numaestra", notif.InvoiceID)
		emoji, title = "💰", "Новая оплата"
		lead = fmt.Sprintf("Заказ #%d оплачен на %s. Генерация запущена.", notif.InvoiceID, formatRubles(notif.AmountKopecks))
	case AdminEventDemoReady:
		subject = fmt.Sprintf("🎧 Готово демо · заказ #%d — Numaestra", notif.InvoiceID)
		emoji, title = "🎧", "Демо готово"
		lead = fmt.Sprintf("По заказу #%d сгенерировано бесплатное демо.", notif.InvoiceID)
	case AdminEventGenerationFailed:
		subject = fmt.Sprintf("⚠️ Ошибка генерации · заказ #%d — Numaestra", notif.InvoiceID)
		emoji, title = "⚠️", "Ошибка генерации"
		lead = fmt.Sprintf("Генерация оплаченного заказа #%d сорвалась — нужно вмешаться вручную.", notif.InvoiceID)
	default:
		subject = fmt.Sprintf("Событие по заказу #%d — Numaestra", notif.InvoiceID)
		emoji, title = "📦", "Событие по заказу"
		lead = fmt.Sprintf("Заказ #%d.", notif.InvoiceID)
	}

	hero := emailHero(emoji, title, "Заказ #"+short)

	var facts strings.Builder
	facts.WriteString(emailParagraph(html.EscapeString(lead)))
	facts.WriteString(adminFactLine("Email клиента", notif.CustomerEmail))
	facts.WriteString(adminFactLine("Телефон", notif.CustomerPhone))
	if notif.Kind == AdminEventGenerationFailed && notif.FailureReason != "" {
		facts.WriteString(adminFactLine("Причина", notif.FailureReason))
	}
	if notif.Brief != "" {
		facts.WriteString(adminFactLine("Бриф", briefSnippet(notif.Brief)))
	}

	body := emailContentRow("28px 32px 8px", facts.String())
	if adminURL != "" {
		body += emailContentRow("0 32px 8px", emailCTAButton(adminURL, "Открыть в админке")+emailCopyLink(adminURL))
	}

	htmlBody = n.emailDocument(subject, hero, body, "Служебное письмо · Numaestra admin")
	textBody = n.buildAdminPlain(notif, lead, adminURL)
	return subject, htmlBody, textBody
}

// adminFactLine — строка «Метка: значение» для тела админского письма (пустые пропускаются).
func adminFactLine(label, value string) string {
	value = strings.TrimSpace(value)
	if value == "" {
		return ""
	}
	return fmt.Sprintf(
		`<p style="margin:0 0 6px;font-size:14px;line-height:1.6;color:rgba(255,255,255,0.7)"><strong style="color:rgba(255,255,255,0.92)">%s:</strong> %s</p>`,
		html.EscapeString(label), html.EscapeString(value),
	)
}

// briefSnippet укорачивает бриф для письма (брифы могут быть длинными).
func briefSnippet(brief string) string {
	brief = strings.TrimSpace(brief)
	const max = 280
	if len([]rune(brief)) > max {
		return string([]rune(brief)[:max]) + "…"
	}
	return brief
}

// formatRubles форматирует копейки в рубли для письма.
func formatRubles(kopecks int64) string {
	return fmt.Sprintf("%d ₽", kopecks/100)
}

// adminOrderURL формирует ссылку на страницу заказа в админке.
func (n *SmtpNotifier) adminOrderURL(orderID string) string {
	if n.publicAppURL == "" || orderID == "" {
		return ""
	}
	return n.publicAppURL + "/admin/orders/" + orderID
}

func (n *SmtpNotifier) buildAdminPlain(notif AdminEventNotification, lead, adminURL string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s\n\n", lead)
	if notif.CustomerEmail != "" {
		fmt.Fprintf(&b, "Email клиента: %s\n", notif.CustomerEmail)
	}
	if notif.CustomerPhone != "" {
		fmt.Fprintf(&b, "Телефон: %s\n", notif.CustomerPhone)
	}
	if notif.Kind == AdminEventGenerationFailed && notif.FailureReason != "" {
		fmt.Fprintf(&b, "Причина: %s\n", notif.FailureReason)
	}
	if notif.Brief != "" {
		fmt.Fprintf(&b, "Бриф: %s\n", briefSnippet(notif.Brief))
	}
	if adminURL != "" {
		fmt.Fprintf(&b, "\nОткрыть в админке:\n%s\n", adminURL)
	}
	b.WriteString("\n—\nNumaestra admin\n")
	return b.String()
}

func (n *SmtpNotifier) send(to, subject, textBody, htmlBody string) error {
	msg := buildMIMEMessage(n.from, n.fromName, to, n.replyTo, subject, textBody, htmlBody)

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

	var tracks strings.Builder
	for i, trackURL := range notif.TrackURLs {
		escaped := html.EscapeString(trackURL)
		fmt.Fprintf(&tracks,
			`<tr>
			  <td style="padding:10px 0;border-bottom:1px solid rgba(255,255,255,0.06)">
			    <a href="%s" style="color:%s;text-decoration:none;font-size:15px;font-weight:600">
			      Вариант %d →
			    </a>
			  </td>
			</tr>`,
			escaped, emailAccent, i+1,
		)
	}

	hero := emailHero("🎵", "Ваша песня готова!",
		fmt.Sprintf("%d версии трека ждут вас на сайте", notif.TracksCount))

	body := emailContentRow("8px 32px 0", emailOrderBadge(shortOrderID(notif.OrderID))) +
		emailContentRow("28px 32px 8px", fmt.Sprintf(`%s
              <table role="presentation" width="100%%" cellpadding="0" cellspacing="0" style="margin-bottom:28px">%s</table>
              %s%s`,
			emailParagraph("Мы закончили генерацию вашей персональной песни. Все версии доступны в плеере на сайте — ссылки действуют бессрочно."),
			tracks.String(),
			emailCTAButton(statusURL, "Слушать все версии"),
			emailCopyLink(statusURL),
		))

	return n.emailDocument(
		"Ваша песня готова — Numaestra",
		hero,
		body,
		"4 версии · готово за ~10 минут · без подписок",
	)
}

func (n *SmtpNotifier) buildFeedbackBody(notif AdminFeedbackNotification) string {
	escaped := strings.ReplaceAll(html.EscapeString(notif.Message), "\n", "<br>")
	hero := emailHero("💬", "Сообщение по заказу", "#"+shortOrderID(notif.OrderID))

	body := emailContentRow("28px 32px 8px",
		fmt.Sprintf(`<p style="margin:0;font-size:16px;line-height:1.65;color:rgba(255,255,255,0.78)">%s</p>`, escaped))

	return n.emailDocument("Сообщение по заказу — Numaestra", hero, body, "")
}

func (n *SmtpNotifier) buildAccessLinkBody(notif AccessLinkNotification) string {
	statusURL := n.orderStatusURL(notif.OrderID, notif.AccessToken)
	hero := emailHero("🔗", "Ссылка на ваш заказ", "По этой ссылке можно оплатить заказ и управлять им")

	body := emailContentRow("8px 32px 0", emailOrderBadge(shortOrderID(notif.OrderID))) +
		emailContentRow("28px 32px 8px", fmt.Sprintf(`%s%s%s`,
			emailParagraph("Вы запросили доступ к заказу на сайте Numaestra. Не пересылайте это письмо — в ссылке есть секретный ключ доступа."),
			emailCTAButton(statusURL, "Открыть заказ"),
			emailCopyLink(statusURL),
		))

	return n.emailDocument("Ссылка на заказ — Numaestra", hero, body, "")
}

// orderStatusURL формирует абсолютную ссылку на страницу статуса заказа.
// UUID в path переживает click-tracking RuSender лучше, чем order_id в query
// (трекер часто обрезает query → фронт падал на старый заказ из localStorage).
//
// Токен в письмах НАМЕРЕННО передаём в query (?token=), а не во fragment (#token=):
// click-трекер RuSender надёжнее сохраняет query, а fragment вообще не доходит до
// сервера и часто теряется при переписывании ссылки. Утечку токена закрывает
// фронт: при открытии /status он сразу перекладывает токен в localStorage и
// вычищает его из адресной строки (history.replaceState), поэтому в историю,
// Referer и аналитику токен не попадает. Share-ссылки (без трекера) используют
// fragment напрямую — см. StatusPage. Менять формат X-Access-Token в API не нужно.
func (n *SmtpNotifier) orderStatusURL(orderID, accessToken string) string {
	path := "/status/" + orderID
	if accessToken != "" {
		path += "?token=" + url.QueryEscape(accessToken)
	}
	if n.publicAppURL == "" {
		return path
	}
	return n.publicAppURL + path
}

func (n *SmtpNotifier) buildPlainBody(notif OrderCompleteNotification) string {
	statusURL := n.orderStatusURL(notif.OrderID, notif.AccessToken)
	var b strings.Builder
	fmt.Fprintf(&b, "Здравствуйте!\n\nВаша персональная песня в Numaestra готова — %d версии трека.\n\n", notif.TracksCount)
	fmt.Fprintf(&b, "Слушать на сайте:\n%s\n\n", statusURL)
	if len(notif.TrackURLs) > 0 {
		b.WriteString("Прямые ссылки на треки:\n")
		for i, u := range notif.TrackURLs {
			fmt.Fprintf(&b, "  Вариант %d: %s\n", i+1, u)
		}
		b.WriteString("\n")
	}
	b.WriteString("—\nNumaestra · персональные песни на заказ\n")
	if n.publicAppURL != "" {
		fmt.Fprintf(&b, "%s\n", n.publicAppURL)
	}
	return b.String()
}

func (n *SmtpNotifier) buildPlainFeedbackBody(notif AdminFeedbackNotification) string {
	return fmt.Sprintf(
		"Сообщение по вашему заказу #%s в Numaestra:\n\n%s\n\n—\nNumaestra\n",
		notif.OrderID,
		notif.Message,
	)
}

func (n *SmtpNotifier) buildPlainAccessLinkBody(notif AccessLinkNotification) string {
	statusURL := n.orderStatusURL(notif.OrderID, notif.AccessToken)
	return fmt.Sprintf(
		"Здравствуйте!\n\nВы запросили ссылку на заказ в Numaestra.\n\nОткрыть заказ:\n%s\n\nНе пересылайте это письмо — в ссылке секретный ключ доступа.\n\n—\nNumaestra\n",
		statusURL,
	)
}

func (n *SmtpNotifier) buildFailedBody(notif OrderFailedNotification) string {
	statusURL := n.orderStatusURL(notif.OrderID, notif.AccessToken)
	hero := emailHero("😔", "Не удалось создать песню", "Заказ #"+shortOrderID(notif.OrderID))

	body := emailContentRow("28px 32px 8px", fmt.Sprintf(`%s%s%s`,
		emailParagraph("К сожалению, при генерации вашей песни произошла ошибка. Мы уже видим проблему и разберёмся в ближайшее время."),
		emailParagraph("Пожалуйста, свяжитесь с нами — мы перегенерируем песню вручную или вернём оплату."),
		emailCTAButton(statusURL, "Открыть заказ"),
	))

	return n.emailDocument("Ошибка генерации — Numaestra", hero, body, "")
}

func (n *SmtpNotifier) buildPlainFailedBody(notif OrderFailedNotification) string {
	statusURL := n.orderStatusURL(notif.OrderID, notif.AccessToken)
	return fmt.Sprintf(
		"Здравствуйте!\n\nК сожалению, при генерации вашей песни (заказ #%s) произошла ошибка.\n\nМы уже видим проблему. Пожалуйста, свяжитесь с нами — мы перегенерируем вручную или вернём оплату.\n\nСтраница заказа:\n%s\n\n—\nNumaestra\n",
		shortOrderID(notif.OrderID),
		statusURL,
	)
}
