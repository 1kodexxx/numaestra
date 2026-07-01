package democlip

import (
	"context"
	"encoding/binary"
	"math"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestNew_DefaultsAndCustom(t *testing.T) {
	p := New("", 0, -1, true)
	if p.ffmpeg != "ffmpeg" {
		t.Errorf("ffmpeg path: %q", p.ffmpeg)
	}
	if p.clipSeconds != 28 {
		t.Errorf("clipSeconds: %d", p.clipSeconds)
	}
	if p.introSkip != 0 {
		t.Errorf("introSkip: %d", p.introSkip)
	}
	if !p.watermark {
		t.Error("watermark должен быть true")
	}

	custom := New("/usr/bin/ffmpeg", 20, 5, false)
	if custom.ffmpeg != "/usr/bin/ffmpeg" || custom.clipSeconds != 20 || custom.introSkip != 5 || custom.watermark {
		t.Errorf("custom processor: %+v", custom)
	}
}

func TestProcess_DownloadError(t *testing.T) {
	p := New("ffmpeg", 10, 0, false)
	_, err := p.Process(context.Background(), "http://127.0.0.1:1/no-such-host")
	if err == nil {
		t.Fatal("ожидали ошибку при недоступном URL")
	}
	if !strings.Contains(err.Error(), "загрузка исходника") {
		t.Errorf("неожиданная ошибка: %v", err)
	}
}

func TestDownloadTemp_ExceedsMaxSize(t *testing.T) {
	body := strings.Repeat("x", maxDownloadBytes+1)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte(body)) //nolint:errcheck
	}))
	defer srv.Close()

	if _, _, err := downloadTemp(context.Background(), srv.URL); err == nil {
		t.Fatal("ожидали ошибку при превышении лимита")
	} else if !strings.Contains(err.Error(), "лимит") {
		t.Errorf("неожиданная ошибка: %v", err)
	}
}

func TestRmsWindows_ZeroSampleRateReturnsNil(t *testing.T) {
	if got := rmsWindows([]byte{0, 0, 0, 0}, 0); got != nil {
		t.Errorf("ожидали nil, получили %v", got)
	}
}

func TestProcess_WithFakeFFmpeg(t *testing.T) {
	ffmpeg := buildFakeFFmpeg(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte("fake-audio-source")) //nolint:errcheck
	}))
	defer srv.Close()

	p := New(ffmpeg, 3, 0, false)
	out, err := p.Process(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("Process: %v", err)
	}
	if len(out) == 0 {
		t.Fatal("ожидали непустой mp3")
	}
}

func buildFakeFFmpeg(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	src := filepath.Join(dir, "main.go")
	const fakeMain = `package main

import (
	"encoding/binary"
	"os"
	"strings"
)

func main() {
	args := strings.Join(os.Args, " ")
	if strings.Contains(args, "s16le") {
		sr := 11025
		for sec := 0; sec < 12; sec++ {
			amp := int16(100)
			if sec >= 5 && sec < 8 {
				amp = 12000
			}
			for i := 0; i < sr; i++ {
				b := make([]byte, 2)
				binary.LittleEndian.PutUint16(b, uint16(amp))
				os.Stdout.Write(b)
			}
		}
		return
	}
	if strings.Contains(args, "mp3") {
		os.Stdout.Write([]byte("fake-mp3-output-bytes"))
		return
	}
	os.Exit(1)
}
`
	if err := os.WriteFile(src, []byte(fakeMain), 0o644); err != nil {
		t.Fatalf("write fake ffmpeg source: %v", err)
	}
	out := filepath.Join(dir, "ffmpeg")
	if runtime.GOOS == "windows" {
		out += ".exe"
	}
	if err := exec.Command("go", "build", "-o", out, src).Run(); err != nil {
		t.Fatalf("build fake ffmpeg: %v", err)
	}
	return out
}

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

func TestHookScore_FavorsLoudAndVocal(t *testing.T) {
	// Секунда 1: громкий инструментал без вокала (high full, low vocal).
	// Секунда 2: тихий куплет с вокалом (low full, high vocal).
	// Секунда 3: припев — громко И с вокалом (high both) → должен победить.
	full := []float64{9, 1, 9}
	vocal := []float64{1, 9, 9}
	got := hookScore(full, vocal)
	want := []float64{9, 9, 81}
	if len(got) != len(want) {
		t.Fatalf("длина: got %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("hookScore[%d] = %v, хотели %v", i, got[i], want[i])
		}
	}
	// Именно припев (индекс 2) — максимум оценки.
	if bestWindowStart(got, 1, 0) != 2 {
		t.Errorf("ожидали, что победит припев (индекс 2)")
	}
}

func TestHookScore_DiffLengths_UsesMin(t *testing.T) {
	if got := hookScore([]float64{1, 2, 3}, []float64{4, 5}); len(got) != 2 {
		t.Errorf("ожидали длину min=2, получили %d", len(got))
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

func TestDownloadTemp_Success(t *testing.T) {
	body := strings.Repeat("audio-bytes", 100)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Write([]byte(body)) //nolint:errcheck
	}))
	defer srv.Close()

	path, cleanup, err := downloadTemp(context.Background(), srv.URL)
	if err != nil {
		t.Fatalf("downloadTemp: %v", err)
	}
	defer cleanup()

	got, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("чтение временного файла: %v", err)
	}
	if string(got) != body {
		t.Errorf("содержимое не совпало: получили %d байт, ожидали %d", len(got), len(body))
	}

	cleanup()
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Error("временный файл должен быть удалён после cleanup")
	}
}

func TestDownloadTemp_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "gone", http.StatusNotFound)
	}))
	defer srv.Close()

	if _, _, err := downloadTemp(context.Background(), srv.URL); err == nil {
		t.Error("ожидали ошибку при HTTP 404")
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
