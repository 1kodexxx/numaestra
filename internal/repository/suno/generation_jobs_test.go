package sunorepo

import (
	"strings"
	"testing"

	"github.com/numaestra/numaestra/internal/domain"
	"github.com/numaestra/numaestra/pkg/suno"
)

func TestIsStructuredLyrics(t *testing.T) {
	if !suno.IsStructuredLyricsText("[Verse 1]\nТекст куплета\n[Chorus]\nПрипев") {
		t.Error("ожидали structured lyrics")
	}
	if suno.IsStructuredLyricsText("Create a happy pop song about birthday") {
		t.Error("английский промпт квиза не должен считаться lyrics")
	}
}

func TestGenerationJobs_TwoLyricVariants(t *testing.T) {
	jobs := generationJobs(domain.MusicGenerationRequest{
		Briefs:     []string{"prompt A", "prompt B"},
		TrackCount: domain.DefaultTrackCount,
	})
	if len(jobs) != 2 {
		t.Fatalf("ожидали 2 задачи, получили %d", len(jobs))
	}
	if jobs[0].Description != "prompt A" || jobs[1].Description != "prompt B" {
		t.Errorf("разные промпты не проброшены: %+v", jobs)
	}
}

func TestGenerationJobs_LLMVariantsUseCustomMode(t *testing.T) {
	lyrics := "[Verse 1]\nСтрока\n[Chorus]\nПрипев"
	jobs := generationJobs(domain.MusicGenerationRequest{
		Briefs:     []string{lyrics, lyrics + " alt"},
		TrackCount: domain.DefaultTrackCount,
	})
	if len(jobs) != 2 {
		t.Fatalf("ожидали 2 задачи, получили %d", len(jobs))
	}
	if jobs[0].Lyrics == "" || jobs[0].Description != "" {
		t.Errorf("ожидали Custom Mode с Lyrics, получили %+v", jobs[0])
	}
}

func TestGenerationJobs_LegacySingleBrief(t *testing.T) {
	jobs := generationJobs(domain.MusicGenerationRequest{
		Brief:      "один текст",
		TrackCount: domain.DefaultTrackCount,
	})
	if len(jobs) != 2 {
		t.Fatalf("legacy: ожидали 2 задачи с одним brief, получили %d", len(jobs))
	}
	if jobs[0].Description != "один текст" || jobs[1].Description != "один текст" {
		t.Errorf("legacy: обе задачи должны использовать один brief, %+v", jobs)
	}
}

func TestGenerationJobs_EncodedPromptSplitsTags(t *testing.T) {
	encoded := suno.EncodePrompt("modern pop, male vocals", "Russian birthday song for Kolya.")
	jobs := generationJobs(domain.MusicGenerationRequest{
		Briefs:     []string{encoded},
		TrackCount: 2,
	})
	if len(jobs) != 1 {
		t.Fatalf("ожидали 1 задачу, получили %d", len(jobs))
	}
	if jobs[0].Tags != "modern pop, male vocals" {
		t.Errorf("tags: %q", jobs[0].Tags)
	}
	if jobs[0].Description == "" {
		t.Errorf("ожидали description, получили %+v", jobs[0])
	}
}

func TestMusicInputFromBrief_PrefersLyrics(t *testing.T) {
	in := musicInputFromBrief("[Chorus]\nHi", "pop", false)
	if in.Lyrics == "" || in.Description != "" {
		t.Errorf("ожидали Lyrics, получили %+v", in)
	}
	if !strings.Contains(in.Lyrics, "[Chorus]") {
		t.Error("lyrics not preserved")
	}
}
