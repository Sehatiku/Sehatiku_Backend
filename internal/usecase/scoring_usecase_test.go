package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"sehatiku-backend/internal/entity"
	"sehatiku-backend/internal/gateway/ml"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

// ── Mocks ────────────────────────────────────────────────────────────────────

type mockDailyFeatureRepo struct {
	df         *entity.DailyFeature
	computeErr error
	created    *entity.DailyFeature
}

func (m *mockDailyFeatureRepo) ComputeRoll7(_ *gorm.DB, _ string, _ time.Time) (*entity.DailyFeature, error) {
	if m.computeErr != nil {
		return nil, m.computeErr
	}
	return m.df, nil
}

func (m *mockDailyFeatureRepo) Upsert(_ *gorm.DB, df *entity.DailyFeature) error {
	if df.ID == "" {
		df.ID = "df-1" // simulasi baris baru (BeforeCreate) / id baris existing
	}
	m.created = df
	return nil
}

type mockRiskScoreRepo struct{ created *entity.RiskScore }

func (m *mockRiskScoreRepo) Create(_ *gorm.DB, rs *entity.RiskScore) error {
	m.created = rs
	return nil
}

type mockScoringPatientRepo struct{ p *entity.Patient }

func (m *mockScoringPatientRepo) FindByID(_ *gorm.DB, _ string) (*entity.Patient, error) {
	return m.p, nil
}

type mockScoringBaselineRepo struct {
	b   *entity.PatientClinicalBaseline
	err error
}

func (m *mockScoringBaselineRepo) FindLatestByPatient(_ *gorm.DB, _ string) (*entity.PatientClinicalBaseline, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.b, nil
}

type mockMLScorer struct {
	gotReq ml.PredictRequest
	res    *ml.PredictResult
	err    error
}

func (m *mockMLScorer) PredictHealthScore(_ context.Context, req ml.PredictRequest) (*ml.PredictResult, error) {
	m.gotReq = req
	if m.err != nil {
		return nil, m.err
	}
	return m.res, nil
}

// ── Tests ────────────────────────────────────────────────────────────────────

func TestScorePatient_HappyPath(t *testing.T) {
	dob := time.Date(1971, 1, 1, 0, 0, 0, 0, time.UTC)
	df := &entity.DailyFeature{
		GlucoseMeanRoll7: 160, GlucoseCVRoll7: 0.18, SystolicRoll7: 140,
		SodiumRoll7: 1500, SleepRoll7: 5.5, ActivityPctRoll7: 0.1,
		StressRoll7: 35, CarbsRoll7: 200,
	}
	dfRepo := &mockDailyFeatureRepo{df: df}
	rsRepo := &mockRiskScoreRepo{}
	scorer := &mockMLScorer{res: &ml.PredictResult{
		HealthScore:  38.6,
		Status:       "bahaya",
		StatusLabel:  "Parah",
		Message:      "Ada beberapa hal yang perlu diperhatikan.",
		TopPenalties: []string{"Gula darah rata-rata Anda cukup tinggi.", "Tensi rata-rata Anda tinggi."},
	}}

	uc := &ScoringUseCase{
		DB:               nil,
		DailyFeatureRepo: dfRepo,
		RiskScoreRepo:    rsRepo,
		PatientRepo:      &mockScoringPatientRepo{p: &entity.Patient{Sex: "male", DateOfBirth: &dob}},
		BaselineRepo: &mockScoringBaselineRepo{b: &entity.PatientClinicalBaseline{
			BMI: 32.0, EGFR: 55, HbA1cPct: 9.5, SystolicBPMmhg: 160,
		}},
		ML:  scorer,
		Log: zap.NewNop(),
	}

	res, err := uc.ScorePatient(context.Background(), "p1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Payload baseline dirakit benar (statis, dari clinical baseline + demografi).
	if scorer.gotReq.Baseline.Sex != "male" {
		t.Errorf("Sex = %q; want male", scorer.gotReq.Baseline.Sex)
	}
	if scorer.gotReq.Baseline.BMI != 32.0 {
		t.Errorf("BMI = %v; want 32.0", scorer.gotReq.Baseline.BMI)
	}
	if scorer.gotReq.Baseline.SystolicBPmmHg != 160 {
		t.Errorf("SystolicBPmmHg = %v; want 160", scorer.gotReq.Baseline.SystolicBPmmHg)
	}
	// Payload daily diambil dari roll-7 (dinamis).
	if scorer.gotReq.Daily7DAverage.GlucoseMeanRoll7 != 160 {
		t.Errorf("GlucoseMeanRoll7 = %v; want 160", scorer.gotReq.Daily7DAverage.GlucoseMeanRoll7)
	}
	if scorer.gotReq.Daily7DAverage.CarbsRoll7 != 200 {
		t.Errorf("CarbsRoll7 = %v; want 200", scorer.gotReq.Daily7DAverage.CarbsRoll7)
	}

	// risk_scores tersimpan: skor dibulatkan, status, mode, link daily_feature, penalties.
	if rsRepo.created == nil {
		t.Fatal("risk_score tidak tersimpan")
	}
	if rsRepo.created.Score != 39 {
		t.Errorf("Score = %d; want 39 (round 38.6)", rsRepo.created.Score)
	}
	if rsRepo.created.Status != "bahaya" {
		t.Errorf("Status = %q; want bahaya", rsRepo.created.Status)
	}
	if rsRepo.created.ScoringMode != "cohort" {
		t.Errorf("ScoringMode = %q; want cohort", rsRepo.created.ScoringMode)
	}
	if rsRepo.created.DailyFeatureID != "df-1" {
		t.Errorf("DailyFeatureID = %q; want df-1", rsRepo.created.DailyFeatureID)
	}
	var penalties []string
	if err := json.Unmarshal(rsRepo.created.TopFactors, &penalties); err != nil {
		t.Fatalf("TopFactors bukan JSON valid: %v", err)
	}
	if len(penalties) != 2 {
		t.Errorf("penalties len = %d; want 2", len(penalties))
	}

	if res.HealthScore != 38.6 {
		t.Errorf("returned HealthScore = %v; want 38.6", res.HealthScore)
	}
}

func TestScorePatient_NoBaseline(t *testing.T) {
	uc := &ScoringUseCase{
		DB:               nil,
		DailyFeatureRepo: &mockDailyFeatureRepo{df: &entity.DailyFeature{}},
		RiskScoreRepo:    &mockRiskScoreRepo{},
		PatientRepo:      &mockScoringPatientRepo{p: &entity.Patient{Sex: "female"}},
		BaselineRepo:     &mockScoringBaselineRepo{err: errors.New("not found")},
		ML:               &mockMLScorer{res: &ml.PredictResult{}},
		Log:              zap.NewNop(),
	}

	_, err := uc.ScorePatient(context.Background(), "p1")
	if !errors.Is(err, ErrNoBaseline) {
		t.Fatalf("err = %v; want wrap of ErrNoBaseline", err)
	}
}
