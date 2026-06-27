package repository

import (
	"encoding/json"
	"fmt"
	"time"

	"gorm.io/gorm"
)

// PatientRiskRow adalah baris risk_scores terbaru milik satu pasien.
type PatientRiskRow struct {
	Score      int
	Status     string
	TopFactors json.RawMessage
	ScoredAt   time.Time
}

// GlucoseRow adalah pengukuran gula darah terakhir dari health_logs.
type GlucoseRow struct {
	Value      float64
	MeasuredAt time.Time
}

// BPRow adalah pengukuran tekanan darah terakhir dari health_logs.
// ValueJSONB berisi {"systolic": N, "diastolic": N} (lihat docs/erd.md konvensi bp).
type BPRow struct {
	ValueJSONB json.RawMessage
	MeasuredAt time.Time
}

type PatientDashboardRepository struct{}

func (r *PatientDashboardRepository) GetLatestRisk(db *gorm.DB, patientID string) (*PatientRiskRow, error) {
	var rows []PatientRiskRow
	err := db.Raw(`
		SELECT score, status, top_factors, scored_at
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

func (r *PatientDashboardRepository) GetLatestGlucose(db *gorm.DB, patientID string) (*GlucoseRow, error) {
	var rows []struct {
		Value      float64
		MeasuredAt time.Time
	}
	err := db.Raw(`
		SELECT value_numeric AS value, measured_at
		FROM health_logs
		WHERE patient_id = ? AND metric_type = 'glucose' AND value_numeric IS NOT NULL
		ORDER BY measured_at DESC
		LIMIT 1
	`, patientID).Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("getting latest glucose: %w", err)
	}
	if len(rows) == 0 {
		return nil, nil
	}
	return &GlucoseRow{Value: rows[0].Value, MeasuredAt: rows[0].MeasuredAt}, nil
}

func (r *PatientDashboardRepository) GetLatestBP(db *gorm.DB, patientID string) (*BPRow, error) {
	var rows []struct {
		ValueJSONB json.RawMessage
		MeasuredAt time.Time
	}
	err := db.Raw(`
		SELECT value_jsonb, measured_at
		FROM health_logs
		WHERE patient_id = ? AND metric_type = 'bp' AND value_jsonb IS NOT NULL
		ORDER BY measured_at DESC
		LIMIT 1
	`, patientID).Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("getting latest blood pressure: %w", err)
	}
	if len(rows) == 0 {
		return nil, nil
	}
	return &BPRow{ValueJSONB: rows[0].ValueJSONB, MeasuredAt: rows[0].MeasuredAt}, nil
}

// GetLogDatesSince mengembalikan daftar tanggal distinct (measured_at::date) yang punya
// minimal satu health_log sejak `since`, terurut menurun. Dipakai usecase untuk menghitung
// logged_today dan streak.
func (r *PatientDashboardRepository) GetLogDatesSince(db *gorm.DB, patientID string, since time.Time) ([]time.Time, error) {
	var dates []time.Time
	err := db.Raw(`
		SELECT DISTINCT measured_at::date AS log_date
		FROM health_logs
		WHERE patient_id = ? AND measured_at >= ?
		ORDER BY log_date DESC
	`, patientID, since).Scan(&dates).Error
	if err != nil {
		return nil, fmt.Errorf("getting log dates: %w", err)
	}
	return dates, nil
}
