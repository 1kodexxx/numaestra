package democlip

import (
	"encoding/binary"
	"math"
	"testing"
)

func TestBestWindowStart_PicksLoudestWindow(t *testing.T) {
	// Тихое интро, громкий «припев» в середине, тихий хвост.
	rms := []float64{0, 0, 0, 1, 1, 9, 9, 9, 1, 1, 0, 0}
	got := bestWindowStart(rms, 3, 0)
	if got != 5 {
		t.Errorf("ожидали старт 5 (самое громкое окно), получили %d", got)
	}
}

func TestBestWindowStart_RespectsIntroSkip(t *testing.T) {
	// Самый громкий участок в самом начале, но интро пропускаем.
	rms := []float64{9, 9, 9, 1, 1, 1, 5, 5, 5, 1}
	got := bestWindowStart(rms, 3, 3)
	if got < 3 {
		t.Errorf("старт должен быть не раньше introSkip=3, получили %d", got)
	}
	if got != 6 {
		t.Errorf("ожидали старт 6 (громкое окно после интро), получили %d", got)
	}
}

func TestBestWindowStart_ShortTrack_ReturnsZero(t *testing.T) {
	if got := bestWindowStart([]float64{1, 2}, 5, 0); got != 0 {
		t.Errorf("для трека короче окна ожидали 0, получили %d", got)
	}
	if got := bestWindowStart(nil, 5, 0); got != 0 {
		t.Errorf("для пустого RMS ожидали 0, получили %d", got)
	}
}

func TestBestWindowStart_IntroSkipTooLarge_FallsBack(t *testing.T) {
	rms := []float64{1, 2, 3, 4, 5}
	// introSkip больше допустимого старта → не должно выкинуть всё, берём с 0.
	got := bestWindowStart(rms, 3, 100)
	if got < 0 || got > len(rms)-3 {
		t.Errorf("старт вне допустимого диапазона: %d", got)
	}
}

func TestRmsWindows_ComputesPerSecond(t *testing.T) {
	sr := 4 // маленькая частота для теста: 4 сэмпла = 1 секунда
	// 2 секунды: первая — амплитуда 100, вторая — тишина.
	pcm := make([]byte, 0, sr*2*2)
	put := func(v int16) {
		b := make([]byte, 2)
		binary.LittleEndian.PutUint16(b, uint16(v))
		pcm = append(pcm, b...)
	}
	for i := 0; i < sr; i++ {
		put(100)
	}
	for i := 0; i < sr; i++ {
		put(0)
	}

	out := rmsWindows(pcm, sr)
	if len(out) != 2 {
		t.Fatalf("ожидали 2 секунды, получили %d", len(out))
	}
	if math.Abs(out[0]-100) > 0.001 {
		t.Errorf("RMS первой секунды ≈ 100, получили %f", out[0])
	}
	if out[1] != 0 {
		t.Errorf("RMS тишины должен быть 0, получили %f", out[1])
	}
}
