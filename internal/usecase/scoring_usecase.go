package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"time"

	"sehatiku-backend/internal/entity"
	"sehatiku-backend/internal/gateway/ml"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

// ErrNoBaseline is returned when a patient has no clinical baseline yet, so the ML
// payload can't be assembled.
var ErrNoBaseline = errors.New("pasien belum memiliki baseline klinis")

type dailyFeatureRepository interface {
	ComputeRoll7(db *gorm.DB, patientID string, asOf time.Time) (*entity.DailyFeature, error)
	Create(db *gorm.DB, df *entity.DailyFeature) error
}

type riskScoreRepository interface {
	Create(db *gorm.DB, score *entity.RiskScore) error
}

type scoringPatientRepository interface {
	FindByID(db *gorm.DB, id string) (*entity.Patient, error)
}

type clinicalBaselineRepository interface {
	FindLatestByPatient(db *gorm.DB, patientID string) (*entity.PatientClinicalBaseline, error)
}

type mlScorer interface {
	PredictHealthScore(ctx context.Context, req ml.PredictRequest) (*ml.PredictResult, error)
}

// ScoringUseCase ties together the nightly/on-open health-score flow:
// roll-7 (SQL) -> daily_features -> ML /predict_health_score -> risk_scores.
type ScoringUseCase struct {
	DB               *gorm.DB
	DailyFeatureRepo dailyFeatureRepository
	RiskScoreRepo    riskScoreRepository
	PatientRepo      scoringPatientRepository
	BaselineRepo     clinicalBaselineRepository
	ML               mlScorer
	Log              *zap.Logger
}

// ScorePatient computes today's features, calls the ML service, and stores the result.
func (u *ScoringUseCase) ScorePatient(ctx context.Context, patientID string) (*ml.PredictResult, error) {
	now := time.Now()

	// 1. roll-7 features from health_logs, persisted to daily_features.
	df, err := u.DailyFeatureRepo.ComputeRoll7(u.DB, patientID, now)
	if err != nil {
		return nil, err
	}
	if err := u.DailyFeatureRepo.Create(u.DB, df); err != nil {
		return nil, err
	}

	// 2. baseline half of the payload (patient demographics + latest clinical baseline).
	patient, err := u.PatientRepo.FindByID(u.DB, patientID)
	if err != nil {
		return nil, fmt.Errorf("loading patient: %w", err)
	}
	baseline, err := u.BaselineRepo.FindLatestByPatient(u.DB, patientID)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrNoBaseline, err)
	}

	req := ml.PredictRequest{
		Baseline: ml.Baseline{
			AgeYears:       scoringAgeFromDOB(patient.DateOfBirth, now),
			Sex:            patient.Sex, // "male"/"female" — accepted by the ML service
			BMI:            baseline.BMI,
			EGFR:           baseline.EGFR,
			HbA1cPct:       baseline.HbA1cPct,
			SystolicBPmmHg: float64(baseline.SystolicBPMmhg),
		},
		Daily7DAverage: ml.DailyAverage{
			GlucoseMeanRoll7: df.GlucoseMeanRoll7,
			GlucoseCVRoll7:   df.GlucoseCVRoll7,
			SystolicRoll7:    df.SystolicRoll7,
			SodiumRoll7:      df.SodiumRoll7,
			SleepRoll7:       df.SleepRoll7,
			ActivityPctRoll7: df.ActivityPctRoll7,
			StressRoll7:      df.StressRoll7,
			CarbsRoll7:       df.CarbsRoll7,
		},
	}

	// 3. ML prediction.
	res, err := u.ML.PredictHealthScore(ctx, req)
	if err != nil {
		return nil, err
	}

	// 4. persist to risk_scores.
	penalties, _ := json.Marshal(res.TopPenalties)
	rs := &entity.RiskScore{
		PatientID:      patientID,
		DailyFeatureID: df.ID,
		Score:          int(math.Round(res.HealthScore)), // entity is int; rounds the float
		Status:         res.Status,                        // aman / waswas / bahaya
		ScoringMode:    "cohort",                          // XGBoost cohort model
		// NOTE: SHAP penalties go into top_factors until a dedicated top_penalties
		// JSONB column exists (see ml-backend-integration-contract action items).
		TopFactors: penalties,
		ScoredAt:   now,
	}
	if err := u.RiskScoreRepo.Create(u.DB, rs); err != nil {
		return nil, err
	}

	u.Log.Info("patient scored",
		zap.String("patient_id", patientID),
		zap.Float64("health_score", res.HealthScore),
		zap.String("status", res.Status),
	)
	return res, nil
}

func scoringAgeFromDOB(dob *time.Time, now time.Time) int {
	if dob == nil {
		return 0
	}
	years := now.Year() - dob.Year()
	if now.YearDay() < dob.YearDay() {
		years--
	}
	if years < 0 {
		years = 0
	}
	return years
}

