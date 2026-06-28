package repository

import (
	"database/sql"
	"fmt"
	"time"

	"sehatiku-backend/internal/entity"

	"gorm.io/gorm"
)

// DailyFeatureRepository computes and persists rolling-7 ML features from health_logs.
type DailyFeatureRepository struct{}

// Clinically-neutral fallbacks, mirroring the Python oracle
// (pipeline/daily_aggregator.py DAILY_DEFAULTS) so a sparse 7-day window still yields a
// finite, sane payload for the model.
const (
	defGlucoseMean = 110.0
	defGlucoseCV   = 0.10
	defSystolic    = 120.0
	defSodium      = 1500.0
	defCarbs       = 150.0
	defSleep       = 7.0
	defActivityPct = 0.3
	defStress      = 20.0
)

// roll7SQL aggregates a patient's trailing 7 days of health_logs into the 8 model
// features. Matches the oracle semantics: per-day means, glucose_cv = std/mean, and
// food nutrition summed per day from value_jsonb (only NER-enriched food logs that
// carry carbs_g/sodium_mg contribute — text-only food logs do not).
//
// UNIT CAVEATS (need calibration before trusting the score in production):
//   - activity: health_logs stores MINUTES/day (0..1440); the model was trained on a
//     0..1 activity fraction, so we divide by 1440 here as a rough proxy.
//   - stress:   health_logs uses a 1..10 scale; the model's training distribution was
//     wider (~0..80). This is passed through unscaled — revisit once calibrated.
const roll7SQL = `
WITH win AS (
    SELECT metric_type, value_numeric, value_jsonb, measured_at
    FROM health_logs
    WHERE patient_id = ?
      AND measured_at >= (?::date - INTERVAL '6 days')
      AND measured_at <  (?::date + INTERVAL '1 day')
),
food_daily AS (
    SELECT measured_at::date AS d,
           SUM((value_jsonb->>'carbs_g')::numeric)   AS carbs,
           SUM((value_jsonb->>'sodium_mg')::numeric) AS sodium
    FROM win
    WHERE metric_type = 'food' AND value_jsonb IS NOT NULL
    GROUP BY 1
),
g AS (
    SELECT AVG(value_numeric) AS mean, STDDEV_SAMP(value_numeric) AS sd
    FROM win WHERE metric_type = 'glucose'
)
SELECT
    (SELECT mean FROM g) AS glucose_mean_roll7,
    CASE WHEN (SELECT mean FROM g) > 0
         THEN (SELECT sd FROM g) / (SELECT mean FROM g) END AS glucose_cv_roll7,
    AVG((value_jsonb->>'systolic')::numeric) FILTER (WHERE metric_type = 'bp') AS systolic_roll7,
    (SELECT AVG(sodium) FROM food_daily) AS sodium_roll7,
    (SELECT AVG(carbs)  FROM food_daily) AS carbs_roll7,
    AVG(value_numeric) FILTER (WHERE metric_type = 'sleep') AS sleep_roll7,
    (AVG(value_numeric) FILTER (WHERE metric_type = 'activity')) / 1440.0 AS activity_pct_roll7,
    AVG(value_numeric) FILTER (WHERE metric_type = 'stress') AS stress_roll7
FROM win
`

type roll7Row struct {
	GlucoseMeanRoll7 sql.NullFloat64 `gorm:"column:glucose_mean_roll7"`
	GlucoseCVRoll7   sql.NullFloat64 `gorm:"column:glucose_cv_roll7"`
	SystolicRoll7    sql.NullFloat64 `gorm:"column:systolic_roll7"`
	SodiumRoll7      sql.NullFloat64 `gorm:"column:sodium_roll7"`
	CarbsRoll7       sql.NullFloat64 `gorm:"column:carbs_roll7"`
	SleepRoll7       sql.NullFloat64 `gorm:"column:sleep_roll7"`
	ActivityPctRoll7 sql.NullFloat64 `gorm:"column:activity_pct_roll7"`
	StressRoll7      sql.NullFloat64 `gorm:"column:stress_roll7"`
}

// ComputeRoll7 returns the 8 daily features for a patient as of `asOf` (the row is NOT
// persisted — call Create to save it). Missing values fall back to clinical defaults.
func (r *DailyFeatureRepository) ComputeRoll7(db *gorm.DB, patientID string, asOf time.Time) (*entity.DailyFeature, error) {
	date := asOf.Format("2006-01-02")
	var row roll7Row
	if err := db.Raw(roll7SQL, patientID, date, date).Scan(&row).Error; err != nil {
		return nil, fmt.Errorf("computing roll-7 features: %w", err)
	}

	return &entity.DailyFeature{
		PatientID:        patientID,
		FeatureDate:      asOf,
		GlucoseMeanRoll7: orDefault(row.GlucoseMeanRoll7, defGlucoseMean),
		GlucoseCVRoll7:   orDefault(row.GlucoseCVRoll7, defGlucoseCV),
		SystolicRoll7:    orDefault(row.SystolicRoll7, defSystolic),
		SodiumRoll7:      orDefault(row.SodiumRoll7, defSodium),
		CarbsRoll7:       orDefault(row.CarbsRoll7, defCarbs),
		SleepRoll7:       orDefault(row.SleepRoll7, defSleep),
		ActivityPctRoll7: orDefault(row.ActivityPctRoll7, defActivityPct),
		StressRoll7:      orDefault(row.StressRoll7, defStress),
	}, nil
}

func (r *DailyFeatureRepository) Create(db *gorm.DB, df *entity.DailyFeature) error {
	if err := db.Create(df).Error; err != nil {
		return fmt.Errorf("creating daily feature: %w", err)
	}
	return nil
}

func orDefault(v sql.NullFloat64, def float64) float64 {
	if v.Valid {
		return v.Float64
	}
	return def
}
