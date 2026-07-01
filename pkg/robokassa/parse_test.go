package robokassa

import "testing"

func TestParseAmountKopecks(t *testing.T) {
	tests := []struct {
		in      string
		want    int64
		wantErr bool
	}{
		{"1500.00", 150000, false},
		{"1500.50", 150050, false},
		{"1500", 150000, false},   // без дробной части
		{"0.01", 1, false},        // одна копейка
		{"1500.5", 150050, false}, // один знак после точки = десятки копеек
		{"1500.", 150000, false},  // точка без копеек
		{".50", 50, false},        // без рублёвой части
		{"  100.00  ", 10000, false},
		{"1000000.00", 100000000, false},
		{"100.000000", 10000, false},   // формат OpStateExt: 6 знаков, все нули
		{"1500.500000", 150050, false}, // OpStateExt с копейками
		{"0.010000", 1, false},         // одна копейка в формате OpStateExt
		{"", 0, true},
		{"abc", 0, true},
		{"100.999", 0, true},    // дробнее копейки — ненулевой 3-й разряд
		{"100.005000", 0, true}, // пол-копейки — не поддерживаем
		{"10.ab", 0, true},
	}
	for _, tt := range tests {
		got, err := ParseAmountKopecks(tt.in)
		if tt.wantErr {
			if err == nil {
				t.Errorf("ParseAmountKopecks(%q): ожидали ошибку", tt.in)
			}
			continue
		}
		if err != nil {
			t.Errorf("ParseAmountKopecks(%q): неожиданная ошибка %v", tt.in, err)
			continue
		}
		if got != tt.want {
			t.Errorf("ParseAmountKopecks(%q) = %d, хотели %d", tt.in, got, tt.want)
		}
	}
}

// Round-trip: FormatAmount и ParseAmountKopecks должны быть согласованы.
func TestFormatParse_RoundTrip(t *testing.T) {
	for _, kop := range []int64{1, 99, 100, 150000, 150050, 100000000} {
		s := FormatAmount(kop)
		got, err := ParseAmountKopecks(s)
		if err != nil {
			t.Fatalf("ParseAmountKopecks(%q): %v", s, err)
		}
		if got != kop {
			t.Errorf("round-trip для %d: FormatAmount=%q → ParseAmountKopecks=%d", kop, s, got)
		}
	}
}

func TestFormatAmount_Negative(t *testing.T) {
	if got := FormatAmount(-150050); got != "-1500.50" {
		t.Errorf("FormatAmount(-150050) = %q, хотели -1500.50", got)
	}
}
