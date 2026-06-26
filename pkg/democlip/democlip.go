// Package democlip собирает «сочный» демо-фрагмент песни: выбирает самый
// энергичный участок (как правило припев), обрезает его, добавляет мягкие fade и
// ненавязчивый водяной знак. Реализация поверх ffmpeg (вызов через os/exec).
//
// Процессор устойчив к сбоям: вызывающая сторона трактует ошибку как сигнал
// деградировать до полного клипа (Фаза 1), поэтому демо не ломается, даже если
// ffmpeg недоступен.
package democlip

import (
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"os/exec"
)

// analysisSampleRate — частота для анализа энергии (моно). Низкая достаточно для
// оценки громкости по секундам и быстро декодируется.
const analysisSampleRate = 11025

// maxDownloadBytes — верхняя граница размера скачиваемого исходника (защита от
// аномально больших файлов). Песня Suno — единицы мегабайт.
const maxDownloadBytes = 40 << 20 // 40 МБ

// Processor вырезает и оформляет демо-фрагмент.
type Processor struct {
	ffmpeg      string
	clipSeconds int
	introSkip   int
	watermark   bool
}

// New создаёт процессор. ffmpegPath пустой → "ffmpeg" из PATH.
func New(ffmpegPath string, clipSeconds, introSkip int, watermark bool) *Processor {
	if ffmpegPath == "" {
		ffmpegPath = "ffmpeg"
	}
	if clipSeconds <= 0 {
		clipSeconds = 28
	}
	if introSkip < 0 {
		introSkip = 0
	}
	return &Processor{ffmpeg: ffmpegPath, clipSeconds: clipSeconds, introSkip: introSkip, watermark: watermark}
}

// Process скачивает аудио по sourceURL один раз во временный файл, выбирает самый
// энергичный участок и возвращает готовый mp3 (обрезанный, с fade и, опционально,
// водяным знаком). Один локальный файл — одна сетевая загрузка и быстрый точный
// seek для обрезки (вместо двойного http-чтения).
func (p *Processor) Process(ctx context.Context, sourceURL string) ([]byte, error) {
	path, cleanup, err := downloadTemp(ctx, sourceURL)
	if err != nil {
		return nil, fmt.Errorf("democlip: загрузка исходника: %w", err)
	}
	defer cleanup()

	rms, err := p.rmsPerSecond(ctx, path)
	if err != nil {
		return nil, fmt.Errorf("democlip: анализ энергии: %w", err)
	}

	totalSec := len(rms)
	start := bestWindowStart(rms, p.clipSeconds, p.introSkip)
	// Длительность фрагмента не должна вылезать за конец короткого клипа.
	dur := p.clipSeconds
	if totalSec > 0 && start+dur > totalSec {
		dur = totalSec - start
	}
	if dur <= 0 {
		dur = p.clipSeconds
	}

	out, err := p.cut(ctx, path, start, dur)
	if err != nil {
		return nil, fmt.Errorf("democlip: обрезка/оформление: %w", err)
	}
	return out, nil
}

// downloadTemp скачивает sourceURL во временный файл (с лимитом размера) и
// возвращает путь и функцию очистки. ffmpeg затем читает локальный файл — это
// быстрее и надёжнее повторного http-seek.
func downloadTemp(ctx context.Context, sourceURL string) (string, func(), error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, sourceURL, nil)
	if err != nil {
		return "", func() {}, err
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", func() {}, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", func() {}, fmt.Errorf("источник вернул HTTP %d", resp.StatusCode)
	}

	tmp, err := os.CreateTemp("", "demo-src-*.audio")
	if err != nil {
		return "", func() {}, err
	}
	cleanup := func() { tmp.Close(); os.Remove(tmp.Name()) } //nolint:errcheck

	n, err := io.Copy(tmp, io.LimitReader(resp.Body, maxDownloadBytes+1))
	if err != nil {
		cleanup()
		return "", func() {}, err
	}
	if n > maxDownloadBytes {
		cleanup()
		return "", func() {}, fmt.Errorf("исходник превышает лимит %d байт", int64(maxDownloadBytes))
	}
	if err := tmp.Close(); err != nil {
		cleanup()
		return "", func() {}, err
	}
	return tmp.Name(), func() { os.Remove(tmp.Name()) }, nil //nolint:errcheck
}

// rmsPerSecond декодирует аудио в моно PCM и возвращает RMS по каждой секунде.
func (p *Processor) rmsPerSecond(ctx context.Context, src string) ([]float64, error) {
	cmd := exec.CommandContext(ctx, p.ffmpeg,
		"-v", "error", "-i", src,
		"-ac", "1", "-ar", fmt.Sprint(analysisSampleRate), "-f", "s16le", "-")
	raw, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	return rmsWindows(raw, analysisSampleRate), nil
}

// rmsWindows вычисляет RMS на каждую секунду из PCM s16le mono.
func rmsWindows(pcm []byte, sampleRate int) []float64 {
	bytesPerSec := sampleRate * 2
	if bytesPerSec == 0 {
		return nil
	}
	var out []float64
	for off := 0; off+bytesPerSec <= len(pcm); off += bytesPerSec {
		var sumSq float64
		for i := off; i < off+bytesPerSec; i += 2 {
			s := int16(binary.LittleEndian.Uint16(pcm[i : i+2]))
			v := float64(s)
			sumSq += v * v
		}
		out = append(out, math.Sqrt(sumSq/float64(sampleRate)))
	}
	return out
}

// bestWindowStart выбирает старт окна длиной windowSec с максимальной суммарной
// энергией (≈ припев/самый полный участок), пропуская первые introSkip секунд
// интро. Возвращает индекс секунды начала.
func bestWindowStart(rms []float64, windowSec, introSkip int) int {
	n := len(rms)
	if n == 0 || windowSec <= 0 || n <= windowSec {
		return 0
	}
	maxStart := n - windowSec
	lo := introSkip
	if lo > maxStart || lo < 0 {
		lo = 0
	}
	// Префиксные суммы для O(n) поиска самого «громкого» окна.
	prefix := make([]float64, n+1)
	for i := 0; i < n; i++ {
		prefix[i+1] = prefix[i] + rms[i]
	}
	bestStart, bestSum := lo, -1.0
	for s := lo; s <= maxStart; s++ {
		sum := prefix[s+windowSec] - prefix[s]
		if sum > bestSum {
			bestSum = sum
			bestStart = s
		}
	}
	return bestStart
}

// cut вырезает [startSec, startSec+durSec], добавляет fade in/out и (если включён)
// ненавязчивый водяной знак — короткий мягкий тон каждые 8 секунд на низкой
// громкости, чтобы пометить демо, но не портить впечатление.
func (p *Processor) cut(ctx context.Context, src string, startSec, durSec int) ([]byte, error) {
	fadeOutStart := float64(durSec) - 1.5
	if fadeOutStart < 0 {
		fadeOutStart = float64(durSec)
	}

	args := []string{"-v", "error", "-ss", fmt.Sprint(startSec), "-t", fmt.Sprint(durSec), "-i", src}
	if p.watermark {
		// Одинарные кавычки в aevalsrc делают запятые литералами внутри выражения.
		filter := fmt.Sprintf(
			"[0:a]afade=t=in:st=0:d=1.2,afade=t=out:st=%.1f:d=1.5[m];"+
				"aevalsrc='if(lt(mod(t,8),0.18), 0.10*sin(2*PI*1100*t), 0)':s=44100:d=%d[w];"+
				"[m][w]amix=inputs=2:duration=first:normalize=0[a]",
			fadeOutStart, durSec)
		args = append(args, "-filter_complex", filter, "-map", "[a]")
	} else {
		args = append(args, "-af",
			fmt.Sprintf("afade=t=in:st=0:d=1.2,afade=t=out:st=%.1f:d=1.5", fadeOutStart))
	}
	args = append(args, "-ac", "2", "-b:a", "128k", "-f", "mp3", "-")

	cmd := exec.CommandContext(ctx, p.ffmpeg, args...)
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("ffmpeg вернул пустой результат")
	}
	return out, nil
}
