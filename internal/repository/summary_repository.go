package repository

import (
	"fmt"
	"time"

	"gorm.io/gorm"
)

// SummaryRepository menyediakan agregasi read-only atas health_logs untuk endpoint
// ringkasan kesehatan (7/14/30 hari). Tidak menulis apa pun — health_logs insert-only.
// Hari di-bucket pada zona Asia/Jakarta agar konsisten dengan dashboard pasien
// (lihat patient_dashboard_repository.go).
type SummaryRepository struct{}

// WindowAggregatesRaw adalah hasil agregasi satu window. Pointer = NULL (tidak ada
// data metrik tsb di window); count = jumlah log mentah yang berkontribusi.
type WindowAggregatesRaw struct {
	GlucoseCount int      `gorm:"column:glucose_count"`
	GlucoseAvg   *float64 `gorm:"column:glucose_avg"`
	GlucoseMin   *float64 `gorm:"column:glucose_min"`
	GlucoseMax   *float64 `gorm:"column:glucose_max"`

	BPCount  int      `gorm:"column:bp_count"`
	BPSysAvg *float64 `gorm:"column:bp_sys_avg"`
	BPDiaAvg *float64 `gorm:"column:bp_dia_avg"`

	MedCount int      `gorm:"column:med_count"`
	MedRate  *float64 `gorm:"column:med_rate"`

	FoodCount     int      `gorm:"column:food_count"`
	FoodDays      int      `gorm:"column:food_days"`
	FoodKcalSum   *float64 `gorm:"column:food_kcal_sum"`
	FoodCarbsSum  *float64 `gorm:"column:food_carbs_sum"`
	FoodSodiumSum *float64 `gorm:"column:food_sodium_sum"`

	ActivityCount int      `gorm:"column:activity_count"`
	ActivityDays  int      `gorm:"column:activity_days"`
	ActivitySum   *float64 `gorm:"column:activity_sum"`

	SleepCount int      `gorm:"column:sleep_count"`
	SleepAvg   *float64 `gorm:"column:sleep_avg"`

	StressCount int      `gorm:"column:stress_count"`
	StressAvg   *float64 `gorm:"column:stress_avg"`
}

// GetWindowAggregates menghitung seluruh agregat metrik untuk health_logs pasien
// dengan measured_at >= since.
func (r *SummaryRepository) GetWindowAggregates(db *gorm.DB, patientID string, since time.Time) (*WindowAggregatesRaw, error) {
	var rows []WindowAggregatesRaw
	err := db.Raw(`
		SELECT
			count(*) FILTER (WHERE metric_type='glucose' AND value_numeric IS NOT NULL) AS glucose_count,
			avg(value_numeric) FILTER (WHERE metric_type='glucose') AS glucose_avg,
			min(value_numeric) FILTER (WHERE metric_type='glucose') AS glucose_min,
			max(value_numeric) FILTER (WHERE metric_type='glucose') AS glucose_max,

			count(*) FILTER (WHERE metric_type='bp' AND value_jsonb IS NOT NULL) AS bp_count,
			avg((value_jsonb->>'systolic')::numeric)  FILTER (WHERE metric_type='bp') AS bp_sys_avg,
			avg((value_jsonb->>'diastolic')::numeric) FILTER (WHERE metric_type='bp') AS bp_dia_avg,

			count(*) FILTER (WHERE metric_type='med_adherence' AND value_numeric IS NOT NULL) AS med_count,
			avg(value_numeric) FILTER (WHERE metric_type='med_adherence') AS med_rate,

			count(*) FILTER (WHERE metric_type='food' AND value_jsonb IS NOT NULL) AS food_count,
			count(DISTINCT (measured_at AT TIME ZONE 'Asia/Jakarta')::date) FILTER (WHERE metric_type='food' AND value_jsonb IS NOT NULL) AS food_days,
			sum((value_jsonb->>'kcal')::numeric)      FILTER (WHERE metric_type='food') AS food_kcal_sum,
			sum((value_jsonb->>'carbs_g')::numeric)   FILTER (WHERE metric_type='food') AS food_carbs_sum,
			sum((value_jsonb->>'sodium_mg')::numeric) FILTER (WHERE metric_type='food') AS food_sodium_sum,

			count(*) FILTER (WHERE metric_type='activity' AND value_numeric IS NOT NULL) AS activity_count,
			count(DISTINCT (measured_at AT TIME ZONE 'Asia/Jakarta')::date) FILTER (WHERE metric_type='activity' AND value_numeric IS NOT NULL) AS activity_days,
			sum(value_numeric) FILTER (WHERE metric_type='activity') AS activity_sum,

			count(*) FILTER (WHERE metric_type='sleep' AND value_numeric IS NOT NULL) AS sleep_count,
			avg(value_numeric) FILTER (WHERE metric_type='sleep') AS sleep_avg,

			count(*) FILTER (WHERE metric_type='stress' AND value_numeric IS NOT NULL) AS stress_count,
			avg(value_numeric) FILTER (WHERE metric_type='stress') AS stress_avg
		FROM health_logs
		WHERE patient_id = ? AND measured_at >= ?
	`, patientID, since).Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("aggregating health logs window: %w", err)
	}
	if len(rows) == 0 {
		return &WindowAggregatesRaw{}, nil
	}
	return &rows[0], nil
}

// WeightWindowRaw adalah berat awal/akhir + jumlah pengukuran dalam window.
type WeightWindowRaw struct {
	StartKg     *float64 `gorm:"column:start_kg"`
	LatestKg    *float64 `gorm:"column:latest_kg"`
	WeightCount int      `gorm:"column:weight_count"`
}

// GetWeightWindow mengambil berat pertama & terakhir (urut measured_at) di window
// untuk menghitung delta berat.
func (r *SummaryRepository) GetWeightWindow(db *gorm.DB, patientID string, since time.Time) (*WeightWindowRaw, error) {
	var rows []WeightWindowRaw
	err := db.Raw(`
		SELECT
			(array_agg(value_numeric ORDER BY measured_at ASC))[1]  AS start_kg,
			(array_agg(value_numeric ORDER BY measured_at DESC))[1] AS latest_kg,
			count(*) AS weight_count
		FROM health_logs
		WHERE patient_id = ? AND metric_type = 'weight' AND value_numeric IS NOT NULL AND measured_at >= ?
	`, patientID, since).Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("aggregating weight window: %w", err)
	}
	if len(rows) == 0 {
		return &WeightWindowRaw{}, nil
	}
	return &rows[0], nil
}

// GetEarliestLogDate mengembalikan tanggal (WIB) health_log paling awal milik pasien,
// atau nil jika pasien belum pernah mengisi. Dipakai untuk window availability gate.
func (r *SummaryRepository) GetEarliestLogDate(db *gorm.DB, patientID string) (*time.Time, error) {
	var rows []time.Time
	err := db.Raw(`
		SELECT (min(measured_at) AT TIME ZONE 'Asia/Jakarta')::date
		FROM health_logs
		WHERE patient_id = ?
	`, patientID).Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("getting earliest log date: %w", err)
	}
	if len(rows) == 0 || rows[0].IsZero() {
		return nil, nil
	}
	return &rows[0], nil
}

// GetLogDatesSince mengembalikan tanggal distinct (WIB) yang punya minimal satu log
// sejak since, terurut menurun — untuk menghitung logged_days & streak.
func (r *SummaryRepository) GetLogDatesSince(db *gorm.DB, patientID string, since time.Time) ([]time.Time, error) {
	var dates []time.Time
	err := db.Raw(`
		SELECT DISTINCT (measured_at AT TIME ZONE 'Asia/Jakarta')::date AS log_date
		FROM health_logs
		WHERE patient_id = ? AND measured_at >= ?
		ORDER BY log_date DESC
	`, patientID, since).Scan(&dates).Error
	if err != nil {
		return nil, fmt.Errorf("getting log dates: %w", err)
	}
	return dates, nil
}

// SummaryRiskRow adalah risk_score terbaru pasien (konteks opsional narasi).
type SummaryRiskRow struct {
	Score    int       `gorm:"column:score"`
	Status   string    `gorm:"column:status"`
	ScoredAt time.Time `gorm:"column:scored_at"`
}

// GetLatestRisk mengambil satu risk_score terbaru pasien, atau nil bila belum ada.
func (r *SummaryRepository) GetLatestRisk(db *gorm.DB, patientID string) (*SummaryRiskRow, error) {
	var rows []SummaryRiskRow
	err := db.Raw(`
		SELECT score, status, scored_at
		FROM risk_scores
		WHERE patient_id = ?
		ORDER BY scored_at DESC
		LIMIT 1
	`, patientID).Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("getting latest risk score: %w", err)
	}
	if len(rows) == 0 {
		return nil, nil
	}
	return &rows[0], nil
}
