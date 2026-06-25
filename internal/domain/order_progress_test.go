package domain

import (
	"testing"

	"github.com/google/uuid"
)

func TestUpdateGenerationProgress_Monotonic(t *testing.T) {
	o, _ := NewOrder(1, "a@b.c", "", "бриф", "", "", 100)
	o.UpdateGenerationProgress(GenerationPhaseGenerating, 40, 1)
	o.UpdateGenerationProgress(GenerationPhaseGenerating, 30, 0) // откат внутри той же фазы
	if o.GenerationProgress() != 40 {
		t.Errorf("прогресс не должен уменьшаться, получили %d", o.GenerationProgress())
	}
	o.UpdateGenerationProgress(GenerationPhaseGenerating, 55, 2)
	if o.GenerationProgress() != 55 || o.TracksReady() != 2 {
		t.Errorf("ожидали 55%% и 2 трека, получили %d/%d", o.GenerationProgress(), o.TracksReady())
	}
}

func TestEnqueue_SetsQueuedProgress(t *testing.T) {
	o, _ := NewOrder(1, "a@b.c", "", "бриф", "", "", 100)
	_ = o.MarkPaid()
	if err := o.Enqueue(); err != nil {
		t.Fatalf("Enqueue: %v", err)
	}
	if o.GenerationPhase() != GenerationPhaseQueued || o.GenerationProgress() != 3 {
		t.Errorf("ожидали queued/3%%, получили %s/%d", o.GenerationPhase(), o.GenerationProgress())
	}
}

func TestStartProcessing_SetsPreparingProgress(t *testing.T) {
	o, _ := NewOrder(1, "a@b.c", "", "бриф", "", "", 100)
	_ = o.MarkPaid()
	_ = o.Enqueue()
	acc := uuid.New()
	if err := o.StartProcessing(acc); err != nil {
		t.Fatalf("StartProcessing: %v", err)
	}
	if o.GenerationPhase() != GenerationPhasePreparing || o.GenerationProgress() != 10 {
		t.Errorf("ожидали preparing/10%%, получили %s/%d", o.GenerationPhase(), o.GenerationProgress())
	}
}

func TestComplete_SetsFullProgress(t *testing.T) {
	o, _ := NewOrder(1, "a@b.c", "", "бриф", "", "", 100)
	_ = o.MarkPaid()
	_ = o.Enqueue()
	_ = o.StartProcessing(uuid.New())
	tracks := []Track{{ID: uuid.New(), Index: 1, AudioURL: "u1"}, {ID: uuid.New(), Index: 2, AudioURL: "u2"}}
	if err := o.Complete(tracks); err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if o.GenerationPhase() != GenerationPhaseCompleted || o.GenerationProgress() != 100 || o.TracksReady() != 2 {
		t.Errorf("ожидали completed/100/2, получили %s/%d/%d", o.GenerationPhase(), o.GenerationProgress(), o.TracksReady())
	}
}

func TestRequeueForRetry_ResetsProgress(t *testing.T) {
	o, _ := NewOrder(1, "a@b.c", "", "бриф", "", "", 100)
	_ = o.MarkPaid()
	_ = o.Enqueue()
	_ = o.StartProcessing(uuid.New())
	o.UpdateGenerationProgress(GenerationPhaseGenerating, 60, 2)
	_ = o.RequeueForRetry()
	if o.GenerationPhase() != GenerationPhaseQueued || o.GenerationProgress() != 3 || o.TracksReady() != 0 {
		t.Errorf("после requeue ожидали queued/3%%/0 треков, получили %s/%d/%d",
			o.GenerationPhase(), o.GenerationProgress(), o.TracksReady())
	}
}

func TestRegenerate_ResetsProgress(t *testing.T) {
	o, _ := NewOrder(1, "a@b.c", "", "бриф", "", "", 100)
	_ = o.MarkPaid()
	_ = o.Enqueue()
	_ = o.StartProcessing(uuid.New())
	o.UpdateGenerationProgress(GenerationPhaseGenerating, 70, 1)
	_ = o.Fail("timeout")
	if err := o.Regenerate(); err != nil {
		t.Fatalf("Regenerate: %v", err)
	}
	if o.GenerationPhase() != GenerationPhaseQueued || o.GenerationProgress() != 3 || o.TracksReady() != 0 {
		t.Errorf("после regenerate ожидали queued/3%%/0, получили %s/%d/%d",
			o.GenerationPhase(), o.GenerationProgress(), o.TracksReady())
	}
}

func TestSnapshot_RoundTripsProgressFields(t *testing.T) {
	o, _ := NewOrder(1, "a@b.c", "", "бриф", "", "", 100)
	o.UpdateGenerationProgress(GenerationPhaseLyrics, 15, 0)
	restored := RestoreOrder(o.Snapshot())
	if restored.GenerationPhase() != GenerationPhaseLyrics || restored.GenerationProgress() != 15 {
		t.Errorf("прогресс не сохранился в snapshot: %s/%d", restored.GenerationPhase(), restored.GenerationProgress())
	}
}
