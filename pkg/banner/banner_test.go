package banner

import (
	"bytes"
	"strings"
	"testing"
)

func TestPrint_ContainsVersionAndEnv(t *testing.T) {
	var buf bytes.Buffer
	Print(&buf, "1.2.3", "production")
	out := buf.String()

	if !strings.Contains(out, "1.2.3") {
		t.Error("баннер должен содержать версию")
	}
	if !strings.Contains(out, "production") {
		t.Error("баннер должен содержать окружение")
	}
}

func TestPrint_NotEmpty(t *testing.T) {
	var buf bytes.Buffer
	Print(&buf, "0.1.0", "dev")
	if buf.Len() == 0 {
		t.Fatal("баннер не должен быть пустым")
	}
	// Должна присутствовать рамка.
	if !strings.Contains(buf.String(), "╭") {
		t.Error("ожидали символы рамки в выводе")
	}
}

func TestPadRight(t *testing.T) {
	if got := padRight("ab", 5); got != "ab   " {
		t.Errorf("padRight(ab,5) = %q", got)
	}
	// Слишком длинная строка обрезается.
	if got := padRight("abcdef", 3); got != "abc" {
		t.Errorf("padRight(abcdef,3) = %q", got)
	}
}
