package sunorepo

import (
	"github.com/numaestra/numaestra/internal/domain"
	"github.com/numaestra/numaestra/pkg/suno"
)

func musicInputFromBrief(brief, style string, instrumental bool) suno.MusicInput {
	return suno.ResolveMusicInput(brief, style, instrumental)
}

// generationJobs строит список Suno-заданий для запроса.
// При двух Briefs — по одной задаче на текст (2 клипа каждая).
// Иначе legacy: tasksFor(TrackCount) задач с одним Brief.
func generationJobs(req domain.MusicGenerationRequest) []suno.MusicInput {
	briefs := req.Briefs
	if len(briefs) == 0 && req.Brief != "" {
		briefs = []string{req.Brief}
	}

	if len(briefs) >= domain.DefaultLyricVariantCount && req.TrackCount >= domain.DefaultTrackCount {
		jobs := make([]suno.MusicInput, 0, domain.DefaultLyricVariantCount)
		for _, brief := range briefs[:domain.DefaultLyricVariantCount] {
			jobs = append(jobs, musicInputFromBrief(brief, req.Style, req.Instrumental))
		}
		return jobs
	}

	brief := ""
	if len(briefs) > 0 {
		brief = briefs[0]
	}
	numTasks := tasksFor(req.TrackCount)
	jobs := make([]suno.MusicInput, numTasks)
	for i := range jobs {
		jobs[i] = musicInputFromBrief(brief, req.Style, req.Instrumental)
	}
	return jobs
}
