package notify

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"net/mail"
	"strings"
	"time"
)

// buildMIMEMessage формирует RFC 5322 multipart/alternative (text + html).
// Транзакционные заголовки и plain-text снижают риск попадания в спам.
func buildMIMEMessage(from, fromName, to, replyTo, subject, textBody, htmlBody string) string {
	domain := domainFromAddress(from)
	messageID := fmt.Sprintf("<%s@%s>", randomHex(16), domain)
	boundary := fmt.Sprintf("numaestra-%s", randomHex(12))

	var sb strings.Builder
	writeHeader(&sb, "From", formatAddress(fromName, from))
	writeHeader(&sb, "To", to)
	if replyTo != "" {
		writeHeader(&sb, "Reply-To", replyTo)
	}
	writeHeader(&sb, "Subject", encodeHeaderUTF8(subject))
	writeHeader(&sb, "Date", time.Now().Format(time.RFC1123Z))
	writeHeader(&sb, "Message-ID", messageID)
	writeHeader(&sb, "MIME-Version", "1.0")
	writeHeader(&sb, "Auto-Submitted", "auto-generated")
	writeHeader(&sb, "X-Auto-Response-Suppress", "All")
	writeHeader(&sb, "Content-Type", fmt.Sprintf(`multipart/alternative; boundary="%s"`, boundary))
	sb.WriteString("\r\n")

	sb.WriteString("--" + boundary + "\r\n")
	sb.WriteString("Content-Type: text/plain; charset=utf-8\r\n")
	sb.WriteString("Content-Transfer-Encoding: quoted-printable\r\n")
	sb.WriteString("\r\n")
	sb.WriteString(encodeQuotedPrintable(textBody))
	sb.WriteString("\r\n")

	sb.WriteString("--" + boundary + "\r\n")
	sb.WriteString("Content-Type: text/html; charset=utf-8\r\n")
	sb.WriteString("Content-Transfer-Encoding: quoted-printable\r\n")
	sb.WriteString("\r\n")
	sb.WriteString(encodeQuotedPrintable(htmlBody))
	sb.WriteString("\r\n")

	sb.WriteString("--" + boundary + "--\r\n")
	return sb.String()
}

func writeHeader(sb *strings.Builder, name, value string) {
	sb.WriteString(name)
	sb.WriteString(": ")
	sb.WriteString(value)
	sb.WriteString("\r\n")
}

func formatAddress(name, addr string) string {
	if name == "" {
		return addr
	}
	return fmt.Sprintf("%s <%s>", encodeHeaderUTF8(name), addr)
}

func encodeHeaderUTF8(s string) string {
	if s == "" {
		return ""
	}
	for i := 0; i < len(s); i++ {
		if s[i] >= 0x80 {
			return "=?UTF-8?B?" + base64.StdEncoding.EncodeToString([]byte(s)) + "?="
		}
	}
	return s
}

func domainFromAddress(addr string) string {
	parsed, err := mail.ParseAddress(addr)
	if err != nil {
		if at := strings.LastIndex(addr, "@"); at >= 0 && at+1 < len(addr) {
			return addr[at+1:]
		}
		return "localhost"
	}
	if at := strings.LastIndex(parsed.Address, "@"); at >= 0 && at+1 < len(parsed.Address) {
		return parsed.Address[at+1:]
	}
	return "localhost"
}

func defaultReplyTo(from string) string {
	domain := domainFromAddress(from)
	if domain == "" || domain == "localhost" {
		return ""
	}
	return "support@" + domain
}

func randomHex(n int) string {
	b := make([]byte, n)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return fmt.Sprintf("%x", b)
}

// encodeQuotedPrintable кодирует UTF-8 текст для MIME (упрощённый QP).
func encodeQuotedPrintable(s string) string {
	var out strings.Builder
	const lineLimit = 76
	lineLen := 0

	flushSoft := func() {
		out.WriteString("=\r\n")
		lineLen = 0
	}

	for i := 0; i < len(s); i++ {
		b := s[i]
		var chunk string
		switch b {
		case '\r':
			continue
		case '\n':
			out.WriteString("\r\n")
			lineLen = 0
			continue
		case '=', '?', '_', '(', ')', '.', ',', ':', ';', '<', '>', '@', '"', '[', ']', ' ':
			chunk = string([]byte{b})
		default:
			if b < 33 || b > 126 {
				chunk = fmt.Sprintf("=%02X", b)
			} else {
				chunk = string([]byte{b})
			}
		}
		if lineLen+len(chunk) > lineLimit {
			flushSoft()
		}
		out.WriteString(chunk)
		lineLen += len(chunk)
	}
	return out.String()
}
