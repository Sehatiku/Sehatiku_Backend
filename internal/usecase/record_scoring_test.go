package usecase

import (
	"context"
	"strings"
	"testing"
	"time"

	"sehatiku-backend/internal/entity"
	"sehatiku-backend/internal/gateway/ml"
	"sehatiku-backend/internal/model"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

type mockRecordLogRepo struct{ created []*entity.HealthLog }

func (m *mockRecordLogRepo) Create(_ *gorm.DB, log *entity.HealthLog) error {
	m.created = append(m.created, log)
	return nil
}

type mockRecordScorer struct {
	res   *ml.PredictResult
	err   error
	calls int
}

func (m *mockRecordScorer) ScorePatient(_ context.Context, _ string) (*ml.PredictResult, error) {
	m.calls++
	return m.res, m.err
}

// /records: makanan di-enrich (value_jsonb) DAN response menyertakan skor.
func TestCreateRecord_EnrichesFoodAndReturnsScore(t *testing.T) {
	logRepo := &mockRecordLogRepo{}
	ext := &mockFoodExtractor{res: &ml.ExtractResult{Totals: ml.Totals{CarbsG: 80, SodiumMg: 500, Kcal: 600}}}
	scorer := &mockRecordScorer{res: &ml.PredictResult{
		HealthScore: 42.5, Status: "waswas", StatusLabel: "Waswas",
		Message: "Ada beberapa hal yang perlu diperhatikan.", TopPenalties: []string{"Gula darah ..."},
	}}
	uc := &RecordUseCase{DB: nil, LogRepo: logRepo, Extractor: ext, Scorer: scorer, Log: zap.NewNop()}

	bs := 150.0
	req := &model.CreateRecordRequest{BloodSugar: &bs, Meals: "nasi goreng", RecordedAt: time.Now().UTC().Format(time.RFC3339)}
	resp, err := uc.CreateRecord(context.Background(), "p1", req)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var food *entity.HealthLog
	for _, l := range logRepo.created {
		if l.MetricType == "food" {
			food = l
		}
	}
	if food == nil || food.ValueJSONB == nil {
		t.Fatal("food log tidak ter-enrich (value_jsonb nil)")
	}
	if !strings.Contains(*food.ValueJSONB, "carbs_g") {
		t.Errorf("value_jsonb tanpa carbs_g: %s", *food.ValueJSONB)
	}

	if resp.Score == nil {
		t.Fatal("expected score di response")
	}
	if resp.Score.HealthScore != 42.5 || resp.Score.StatusLabel != "Waswas" {
		t.Errorf("score = %+v", resp.Score)
	}
	if scorer.calls != 1 {
		t.Errorf("scorer dipanggil %d kali, want 1", scorer.calls)
	}
}

// Skoring best-effort: bila gagal, catatan tetap tersimpan & score di-omit (nil).
func TestCreateRecord_ScoringFails_StillSaves(t *testing.T) {
	logRepo := &mockRecordLogRepo{}
	scorer := &mockRecordScorer{err: ErrNoBaseline}
	uc := &RecordUseCase{DB: nil, LogRepo: logRepo, Scorer: scorer, Log: zap.NewNop()}

	bs := 150.0
	req := &model.CreateRecordRequest{BloodSugar: &bs, RecordedAt: time.Now().UTC().Format(time.RFC3339)}
	resp, err := uc.CreateRecord(context.Background(), "p1", req)
	if err != nil {
		t.Fatalf("catatan harus tetap tersimpan walau scoring gagal: %v", err)
	}
	if resp.Score != nil {
		t.Errorf("score harus nil saat scoring gagal, got %+v", resp.Score)
	}
	if len(logRepo.created) == 0 {
		t.Fatal("glucose log harusnya tersimpan")
	}
}
