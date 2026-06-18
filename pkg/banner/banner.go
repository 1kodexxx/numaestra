package banner

import (
	"fmt"
	"io"
	"runtime"
	"strings"
	"unicode/utf8"
)

const (
	ansiReset = "\033[0m"
	ansiBold  = "\033[1m"
	orchid    = "\033[38;5;170m"
)

const (
	sideMargin      = 2
	leftValueWidth  = 15
	rightValueWidth = 25
)

var logo = []string{
	`███╗   ██╗██╗   ██╗███╗   ███╗ █████╗ ███████╗███████╗████████╗██████╗  █████╗ `,
	`████╗  ██║██║   ██║████╗ ████║██╔══██╗██╔════╝██╔════╝╚══██╔══╝██╔══██╗██╔══██╗`,
	`██╔██╗ ██║██║   ██║██╔████╔██║███████║█████╗  ███████╗   ██║   ██████╔╝███████║`,
	`██║╚██╗██║██║   ██║██║╚██╔╝██║██╔══██║██╔══╝  ╚════██║   ██║   ██╔══██╗██╔══██║`,
	`██║ ╚████║╚██████╔╝██║ ╚═╝ ██║██║  ██║███████╗███████║   ██║   ██║  ██║██║  ██║`,
	`╚═╝  ╚═══╝ ╚═════╝ ╚═╝     ╚═╝╚═╝  ╚═╝╚══════╝╚══════╝   ╚═╝   ╚═╝  ╚═╝╚═╝  ╚═╝`,
}

var contentWidth = func() int {
	max := 0
	for _, line := range logo {
		if n := utf8.RuneCountInString(line); n > max {
			max = n
		}
	}
	return max
}()

func padRight(s string, width int) string {
	rc := utf8.RuneCountInString(s)
	if rc >= width {
		return string([]rune(s)[:width])
	}
	return s + strings.Repeat(" ", width-rc)
}

func border(left, right string) string {
	return ansiBold + orchid + left + strings.Repeat("─", contentWidth+sideMargin*2) + right + ansiReset
}

func row(content string) string {
	return ansiBold + orchid + "│" + ansiReset + strings.Repeat(" ", sideMargin) +
		content + strings.Repeat(" ", sideMargin) + ansiBold + orchid + "│" + ansiReset
}

func hudRow(leftLabel, leftVal, rightLabel, rightVal string) string {
	leftVal = padRight(leftVal, leftValueWidth)
	rightVal = padRight(rightVal, rightValueWidth)

	leftPlain := "[" + leftLabel + "] " + leftVal
	rightPlain := "[" + rightLabel + "] " + rightVal

	gap := contentWidth - utf8.RuneCountInString(leftPlain) - utf8.RuneCountInString(rightPlain)
	if gap < 1 {
		gap = 1
	}

	var b strings.Builder
	// Ещё немного предвыделения памяти не повредит
	b.Grow(contentWidth + 100)

	b.WriteString(ansiBold + orchid + "[")
	b.WriteString(leftLabel)
	b.WriteString("]")
	b.WriteString(ansiReset)
	b.WriteString(orchid + " ")
	b.WriteString(leftVal)
	b.WriteString(ansiReset)
	b.WriteString(strings.Repeat(" ", gap))
	b.WriteString(ansiBold + orchid + "[")
	b.WriteString(rightLabel)
	b.WriteString("]")
	b.WriteString(ansiReset)
	b.WriteString(orchid + " ")
	b.WriteString(rightVal)
	b.WriteString(ansiReset)

	return b.String()
}

// Print выводит ASCII-баннер Numaestra в стиле cyberpunk HUD.
func Print(w io.Writer, version, env string) {
	fmt.Fprintln(w)
	fmt.Fprintln(w, border("╭", "╮"))

	for _, line := range logo {
		fmt.Fprintln(w, row(ansiBold+orchid+padRight(line, contentWidth)+ansiReset))
	}

	fmt.Fprintln(w, border("├", "┤"))

	// Теперь HUD содержит 3 строки для максимальной киберпанк-информативности
	fmt.Fprintln(w, row(hudRow("SYSTEM", "ONLINE", "SERVICE", "музыка на заказ (Suno)")))
	fmt.Fprintln(w, row(hudRow("CORE  ", version, "ENV    ", env)))
	fmt.Fprintln(w, row(hudRow("GO    ", runtime.Version(), "PLATFRM", runtime.GOOS+"/"+runtime.GOARCH)))

	fmt.Fprintln(w, border("╰", "╯"))
	fmt.Fprintln(w)
}
