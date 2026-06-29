package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"sehatiku-backend/internal/entity"
	"sehatiku-backend/internal/gateway/ml"

	"go.uber.org/zap"
)

type mockFoodExtractor struct {
	res *ml.ExtractResult
	err error
}

func (m *mockFoodExtractor) ExtractChat(_ context.Context, _ string) (*ml.ExtractResult, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.res, nil
}

// Makanan di-enrich: gizi teragregasi (carbs_g/sodium_mg) ditulis ke value_jsonb —
// itulah yang dijumlahkan roll-7 SQL. Tanpa ini, makanan tak pernah sampai ke model.
func TestEnrichFood_PopulatesNutrition(t *testing.T) {
	ext := &mockFoodExtractor{res: &ml.ExtractResult{
		Totals: ml.Totals{Kcal: 838, CarbsG: 152.8, SodiumMg: 42},
		Foods:  []ml.ExtractFood{{Query: "nasi goreng", Matched: "Intip goreng", CarbsG: 62.3}},
	}}
	uc := &HealthLogUseCase{Extractor: ext, Log: zap.NewNop()}
	log := &entity.HealthLog{PatientID: "p1"}

	uc.enrichFood(context.Background(), log, "nasi goreng sama ayam goreng")

	if log.ValueJSONB == nil {
		t.Fatal("ValueJSONB nil; harusnya berisi gizi")
	}
	var got map[string]any
	if err := json.Unmarshal([]byte(*log.ValueJSONB), &got); err != nil {
		t.Fatalf("value_jsonb bukan JSON valid: %v", err)
	}
	if got["carbs_g"] != 152.8 {
		t.Errorf("carbs_g = %v; want 152.8", got["carbs_g"])
	}
	if got["sodium_mg"] != 42.0 {
		t.Errorf("sodium_mg = %v; want 42", got["sodium_mg"])
	}
}

// Bila ML mati, log makanan tetap tersimpan teks-saja (value_jsonb null) — request
// tidak digagalkan; gizi belum terhitung sampai di-enrich ulang.
func TestEnrichFood_MLDown_LeavesTextOnly(t *testing.T) {
	ext := &mockFoodExtractor{err: errors.New("upstream down")}
	uc := &HealthLogUseCase{Extractor: ext, Log: zap.NewNop()}
	log := &entity.HealthLog{PatientID: "p1"}

	uc.enrichFood(context.Background(), log, "nasi goreng")

	if log.ValueJSONB != nil {
		t.Errorf("ValueJSONB = %v; want nil saat ML gagal", *log.ValueJSONB)
	}
}
