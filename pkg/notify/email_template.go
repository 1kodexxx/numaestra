package notify

import (
	"fmt"
	"html"
	"strings"
	"time"
)

const (
	emailAccent     = "#00e5c0"
	emailAccentDark = "#00bfa5"
	emailBgOuter    = "#f3f4f6"
	emailBgCard     = "#0f0f0f"
	emailText       = "#ffffff"
	emailTextMuted  = "rgba(255,255,255,0.55)"
)

// emailBrandHeader — логотип + название Numaestra в шапке письма.
func (n *SmtpNotifier) emailBrandHeader() string {
	home := strings.TrimRight(n.publicAppURL, "/")
	homeEsc := html.EscapeString(home)
	brandName := `<span style="font-size:22px;font-weight:800;letter-spacing:-0.03em;color:#ffffff;vertical-align:middle">Numaestra</span>`

	if home != "" {
		logoURL := html.EscapeString(home + "/email-logo.png")
		return fmt.Sprintf(`
          <table role="presentation" cellpadding="0" cellspacing="0" style="margin:0 auto 20px">
            <tr>
              <td style="padding-right:12px;vertical-align:middle">
                <a href="%s" style="text-decoration:none;display:inline-block;line-height:0">
                  <img src="%s" width="44" height="44" alt="Numaestra" style="display:block;border:0;border-radius:11px;width:44px;height:44px">
                </a>
              </td>
              <td style="vertical-align:middle">
                <a href="%s" style="text-decoration:none">%s</a>
              </td>
            </tr>
          </table>`,
			homeEsc, logoURL, homeEsc, brandName,
		)
	}

	// Dev без PUBLIC_APP_URL — HTML-марка (5 столбиков), как на сайте.
	return fmt.Sprintf(`
          <table role="presentation" cellpadding="0" cellspacing="0" style="margin:0 auto 20px">
            <tr>
              <td style="padding-right:12px;vertical-align:middle">
                <div style="width:44px;height:44px;border-radius:11px;background:#0a0a0a;display:inline-block;text-align:center;line-height:44px">
                  <span style="display:inline-block;vertical-align:middle;font-size:0;line-height:0">
                    <span style="display:inline-block;width:4px;height:14px;background:%s;border-radius:2px;margin:0 1px"></span>
                    <span style="display:inline-block;width:4px;height:22px;background:%s;border-radius:2px;margin:0 1px"></span>
                    <span style="display:inline-block;width:4px;height:32px;background:%s;border-radius:2px;margin:0 1px"></span>
                    <span style="display:inline-block;width:4px;height:22px;background:%s;border-radius:2px;margin:0 1px"></span>
                    <span style="display:inline-block;width:4px;height:14px;background:%s;border-radius:2px;margin:0 1px"></span>
                  </span>
                </div>
              </td>
              <td style="vertical-align:middle">%s</td>
            </tr>
          </table>`,
		emailAccent, emailAccent, emailAccent, emailAccent, emailAccent, brandName,
	)
}

func emailHero(icon, title, subtitle string) string {
	subtitleBlock := ""
	if subtitle != "" {
		subtitleBlock = fmt.Sprintf(`<p style="margin:0;font-size:15px;line-height:1.5;color:%s">%s</p>`,
			emailTextMuted, html.EscapeString(subtitle))
	}
	return fmt.Sprintf(`
          <div style="font-size:40px;line-height:1;margin-bottom:16px">%s</div>
          <h1 style="margin:0 0 10px;font-size:28px;font-weight:800;letter-spacing:-0.03em;color:%s">%s</h1>
          %s`,
		icon, emailText, html.EscapeString(title), subtitleBlock,
	)
}

func emailOrderBadge(shortOrderID string) string {
	return fmt.Sprintf(`
          <table role="presentation" width="100%%" cellpadding="0" cellspacing="0">
            <tr>
              <td style="padding:14px 16px;background:rgba(0,229,192,0.06);border:1px solid rgba(0,229,192,0.16);border-radius:12px">
                <div style="font-size:12px;color:rgba(255,255,255,0.45);margin-bottom:4px">Заказ</div>
                <div style="font-size:14px;font-weight:600;color:#ffffff;font-family:ui-monospace,Consolas,monospace">#%s</div>
              </td>
            </tr>
          </table>`,
		html.EscapeString(shortOrderID),
	)
}

func emailCTAButton(href, label string) string {
	return fmt.Sprintf(`
          <table role="presentation" width="100%%" cellpadding="0" cellspacing="0">
            <tr>
              <td align="center">
                <a href="%s" style="display:inline-block;background:linear-gradient(135deg,%s,%s);color:#080808;text-decoration:none;padding:15px 36px;border-radius:12px;font-size:16px;font-weight:700;letter-spacing:-0.01em">
                  %s
                </a>
              </td>
            </tr>
          </table>`,
		html.EscapeString(href), emailAccent, emailAccentDark, html.EscapeString(label),
	)
}

func emailCopyLink(href string) string {
	esc := html.EscapeString(href)
	return fmt.Sprintf(`
          <p style="margin:24px 0 0;font-size:12px;line-height:1.6;color:rgba(255,255,255,0.35);text-align:center">
            Если кнопка не открывается, скопируйте ссылку:<br>
            <a href="%s" style="color:%s;word-break:break-all">%s</a>
          </p>`, esc, emailAccent, esc)
}

func emailFooter(tagline string) string {
	tag := ""
	if tagline != "" {
		tag = fmt.Sprintf(`<p style="margin:0 0 8px;font-size:12px;color:rgba(255,255,255,0.35)">%s</p>`,
			html.EscapeString(tagline))
	}
	return fmt.Sprintf(`
          <tr>
            <td style="padding:20px 32px 28px;border-top:1px solid rgba(255,255,255,0.06);text-align:center">
              %s
              <p style="margin:0;font-size:12px;color:rgba(255,255,255,0.28)">© %d Numaestra · Персональные песни на заказ</p>
            </td>
          </tr>`,
		tag, time.Now().Year(),
	)
}

func (n *SmtpNotifier) emailDocument(title, heroBlock, innerBody, footerTagline string) string {
	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="ru">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1">
  <title>%s</title>
</head>
<body style="margin:0;padding:0;background:%s;color:#111111;font-family:'Segoe UI',Arial,Helvetica,sans-serif">
  <table role="presentation" width="100%%" cellpadding="0" cellspacing="0" style="background:%s">
    <tr>
      <td align="center" style="padding:40px 16px">
        <table role="presentation" width="600" cellpadding="0" cellspacing="0" style="max-width:600px;width:100%%;background:%s;border:1px solid rgba(255,255,255,0.07);border-radius:20px;overflow:hidden">
          <tr>
            <td style="padding:28px 32px 24px;text-align:center;background:radial-gradient(ellipse 80%% 60%% at 50%% 0%%, rgba(0,229,192,0.14) 0%%, transparent 70%%)">
              %s
              %s
            </td>
          </tr>
          %s
          %s
        </table>
      </td>
    </tr>
  </table>
</body>
</html>`,
		html.EscapeString(title),
		emailBgOuter, emailBgOuter, emailBgCard,
		n.emailBrandHeader(),
		heroBlock,
		innerBody,
		emailFooter(footerTagline),
	)
}

func shortOrderID(full string) string {
	if len(full) > 8 {
		return full[:8]
	}
	return full
}

func emailContentRow(padding, content string) string {
	return fmt.Sprintf(`<tr><td style="padding:%s">%s</td></tr>`, padding, content)
}

func emailParagraph(text string) string {
	return fmt.Sprintf(`<p style="margin:0 0 20px;font-size:15px;line-height:1.65;color:rgba(255,255,255,0.72)">%s</p>`,
		text)
}
