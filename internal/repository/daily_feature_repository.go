package repository

import (
	"database/sql"
	"errors"
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
// UNIT CALIBRATION — menyelaraskan satuan app ke distribusi data latih model
// (diverifikasi atas Dataset-Simulation/sim_timeseries_365k_v3.csv):
//   - activity: app mencatat MENIT/hari; fitur latih `activity_30m` BINER (1 bila
//     >=30 menit/hari, selain itu 0) dan input model = fraksi hari aktif (0..1,
//     rata-rata ~0.33). Kita jumlahkan menit per hari, tandai >=30 menit, lalu rata2.
//   - stress: app skala 1..10; `stress_mean` data latih ~0..82 (rata-rata ~20.8).
//     Di-rescale linear: model = (app-1)/9 * 82  (app 1->0, app 10->82).
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
activity_daily AS (
    SELECT measured_at::date AS d,
           CASE WHEN SUM(value_numeric) >= 30 THEN 1.0 ELSE 0.0 END AS active
    FROM win
    WHERE metric_type = 'activity'
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
    (SELECT AVG(active) FROM activity_daily) AS activity_pct_roll7,
    AVG(((value_numeric - 1) / 9.0) * 82.0) FILTER (WHERE metric_type = 'stress') AS stress_roll7
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

// Upsert menyimpan fitur hari ini dengan menghormati UNIQUE(patient_id, feature_date):
// 1 baris/pasien/hari. Bila skoring on-demand dipanggil berkali-kali di hari yang sama,
// baris yang ada DI-UPDATE (bukan insert baru yang akan melanggar unique). df.ID di-set
// ke id baris riil agar risk_scores.daily_feature_id (FK) menunjuk ke baris yang benar.
func (r *DailyFeatureRepository) Upsert(db *gorm.DB, df *entity.DailyFeature) error {
	day := df.FeatureDate.Format("2006-01-02")
	var existing entity.DailyFeature
	err := db.Where("patient_id = ? AND feature_date = ?::date", df.PatientID, day).First(&existing).Error
	switch {
	case err == nil:
		df.ID = existing.ID
		// Map (bukan struct) supaya nilai nol (mis. activity_pct_roll7 = 0) tetap ditulis.
		updates := map[string]any{
			"glucose_mean_roll7": df.GlucoseMeanRoll7,
			"glucose_cv_roll7":   df.GlucoseCVRoll7,
			"systolic_roll7":     df.SystolicRoll7,
			"sodium_roll7":       df.SodiumRoll7,
			"sleep_roll7":        df.SleepRoll7,
			"activity_pct_roll7": df.ActivityPctRoll7,
			"stress_roll7":       df.StressRoll7,
			"carbs_roll7":        df.CarbsRoll7,
		}
		if err := db.Model(&entity.DailyFeature{}).Where("id = ?", existing.ID).Updates(updates).Error; err != nil {
			return fmt.Errorf("updating daily feature: %w", err)
		}
		return nil
	case errors.Is(err, gorm.ErrRecordNotFound):
		if err := db.Create(df).Error; err != nil {
			return fmt.Errorf("creating daily feature: %w", err)
		}
		return nil
	default:
		return fmt.Errorf("looking up daily feature: %w", err)
	}
}

func orDefault(v sql.NullFloat64, def float64) float64 {
	if v.Valid {
		return v.Float64
	}
	return def
}
