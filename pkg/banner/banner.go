package banner

import (
	"fmt"
	"io"
	"strings"
	"unicode/utf8"
)

// ANSI-коды cyberpunk neon палитры (256-цветной режим терминала).
const (
	ansiReset = "\033[0m"
	ansiBold  = "\033[1m"

	neonCyan    = "\033[38;5;51m"
	neonBlue    = "\033[38;5;39m"
	neonPurple  = "\033[38;5;135m"
	neonPink    = "\033[38;5;161m"
	neonMagenta = "\033[38;5;201m"
	neonAmber   = "\033[38;5;214m"
	neonGreen   = "\033[38;5;48m"
)

// gradient - расширенный 6-ступенчатый переход от электрического циана к неоновому красно-розовому
var gradient = []string{
	"\033[38;5;51m",  // Cyan
	"\033[38;5;45m",  // Light Blue
	"\033[38;5;39m",  // Deep Blue
	"\033[38;5;135m", // Purple
	"\033[38;5;161m", // Hot Pink
	"\033[38;5;196m", // Neon Red
}

// logo - ИСПРАВЛЕННЫЙ ASCII-арт "NUMAESTRA" (шрифт ANSI Shadow).
// Ширина логотипа 79 символов.
var logo = []string{
	`███╗   ██╗██╗   ██╗███╗   ███╗ █████╗ ███████╗███████╗████████╗██████╗  █████╗ `,
	`████╗  ██║██║   ██║████╗ ████║██╔══██╗██╔════╝██╔════╝╚══██╔══╝██╔══██╗██╔══██╗`,
	`██╔██╗ ██║██║   ██║██╔████╔██║███████║█████╗  ███████╗   ██║   ██████╔╝███████║`,
	`██║╚██╗██║██║   ██║██║╚██╔╝██║██╔══██║██╔══╝  ╚════██║   ██║   ██╔══██╗██╔══██║`,
	`██║ ╚████║╚██████╔╝██║ ╚═╝ ██║██║  ██║███████╗███████║   ██║   ██║  ██║██║  ██║`,
	`╚═╝  ╚═══╝ ╚═════╝ ╚═╝     ╚═╝╚═╝  ╚═╝╚══════╝╚══════╝   ╚═╝   ╚═╝  ╚═╝╚═╝  ╚═╝`,
}

// padRight безопасно дополняет строку пробелами до нужной длины или обрезает её.
func padRight(s string, l int) string {
	rc := utf8.RuneCountInString(s)
	if rc >= l {
		return string([]rune(s)[:l])
	}
	return s + strings.Repeat(" ", l-rc)
}

// Print выводит ASCII-баннер Numaestra в стиле Cyberpunk UI.
func Print(w io.Writer, version, env string) {
	fmt.Fprintln(w)

	boxColor := neonPurple
	// Внутренняя ширина рамки теперь 83 символа (79 лого + по 2 пробела по бокам)
	divider := boxColor + "├" + strings.Repeat("─", 83) + "┤" + ansiReset

	// Верхняя граница рамки
	fmt.Fprintln(w, boxColor+"╭"+strings.Repeat("─", 83)+"╮"+ansiReset)

	// Отрисовка логотипа с градиентом и боковыми рамками
	for i, line := range logo {
		color := gradient[i%len(gradient)]
		fmt.Fprintf(w, "%s│%s  %s%s%s  %s│%s\n",
			boxColor, ansiReset,
			ansiBold+color, line, ansiReset,
			boxColor, ansiReset)
	}

	// Разделитель между логотипом и метаданными
	fmt.Fprintln(w, divider)

	// --- Первая строка метаданных (HUD) ---
	leftLabel1 := neonCyan + " SYSTEM " + ansiReset
	leftVal1 := neonGreen + padRight("ONLINE", 15) + ansiReset

	rightLabel1 := neonCyan + " SERVICE " + ansiReset
	rightVal1 := neonMagenta + padRight("музыка на заказ (Suno)", 25) + ansiReset

	// Отступы пересчитаны под ширину 83: 16 пробелов в центре
	fmt.Fprintf(w, "%s│%s  [%s] %s                [%s] %s  %s│%s\n",
		boxColor, ansiReset, leftLabel1, leftVal1, rightLabel1, rightVal1, boxColor, ansiReset)

	// --- Вторая строка метаданных (HUD) ---
	leftLabel2 := neonCyan + " CORE   " + ansiReset
	leftVal2 := neonAmber + padRight(version, 15) + ansiReset

	rightLabel2 := neonCyan + " ENV     " + ansiReset
	rightVal2 := neonAmber + padRight(env, 25) + ansiReset

	fmt.Fprintf(w, "%s│%s  [%s] %s                [%s] %s  %s│%s\n",
		boxColor, ansiReset, leftLabel2, leftVal2, rightLabel2, rightVal2, boxColor, ansiReset)

	// Нижняя граница рамки
	fmt.Fprintln(w, boxColor+"╰"+strings.Repeat("─", 83)+"╯"+ansiReset)
	fmt.Fprintln(w)
}
