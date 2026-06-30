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

// GetLogDatesSince mengembalikan daftar tanggal distinct (zona Asia/Jakarta) yang punya
// minimal satu health_log sejak `since`, terurut menurun. Dipakai usecase untuk menghitung
// logged_today dan streak; tanggal dihitung di WIB agar konsisten dengan endpoint status harian.
func (r *PatientDashboardRepository) GetLogDatesSince(db *gorm.DB, patientID string, since time.Time) ([]time.Time, error) {
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

// GetLastLogAt mengembalikan measured_at health_log terakhir milik pasien, atau nil jika
// pasien belum pernah mengisi sama sekali. Dipakai endpoint status input harian untuk
// menghitung logged_today dan jumlah hari sejak input terakhir (dihitung di WIB oleh usecase).
func (r *PatientDashboardRepository) GetLastLogAt(db *gorm.DB, patientID string) (*time.Time, error) {
	var rows []time.Time
	err := db.Raw(`
		SELECT measured_at
		FROM health_logs
		WHERE patient_id = ?
		ORDER BY measured_at DESC
		LIMIT 1
	`, patientID).Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("getting last log time: %w", err)
	}
	if len(rows) == 0 {
		return nil, nil
	}
	return &rows[0], nil
}

// GetNakesFullName mengambil full_name nakes berdasarkan ID-nya.
// Mengembalikan "" (bukan error) jika nakes tidak ditemukan — caller menangani gracefully.
func (r *PatientDashboardRepository) GetNakesFullName(db *gorm.DB, nakesID string) (string, error) {
	var result struct{ FullName string }
	err := db.Raw(`SELECT full_name FROM nakes WHERE id = ? LIMIT 1`, nakesID).Scan(&result).Error
	if err != nil {
		return "", fmt.Errorf("getting nakes full_name: %w", err)
	}
	return result.FullName, nil
}

// RecordHistoryRaw adalah satu baris data harian agregat per hari dari health_logs,
// dipakai oleh GetRecordHistory untuk mendukung grafik di Patient App.
type RecordHistoryRaw struct {
	LogDate     time.Time `gorm:"column:log_date"`
	BloodSugar  *float64  `gorm:"column:blood_sugar"`
	BpRaw       *string   `gorm:"column:bp_raw"`
	Weight      *float64  `gorm:"column:weight"`
	HealthScore *int      `gorm:"column:health_score"`
}

// GetRecordHistory mengambil riwayat harian (glucose, bp, weight, health score) per hari terbaru.
// Untuk setiap hari, diambil satu nilai terbaru per metrik (DISTINCT ON per hari).
// Query ini bekerja setelah migration 000008 (yang menambah 'weight' ke enum health_metric).
func (r *PatientDashboardRepository) GetRecordHistory(db *gorm.DB, patientID string, limit int) ([]RecordHistoryRaw, error) {
	var rows []RecordHistoryRaw
	err := db.Raw(`
		WITH days AS (
			SELECT DISTINCT measured_at::date AS log_date
			FROM health_logs
			WHERE patient_id = ?
			  AND metric_type IN ('glucose', 'bp', 'weight')
			ORDER BY log_date DESC
			LIMIT ?
		),
		glucose AS (
			SELECT DISTINCT ON (measured_at::date) measured_at::date AS log_date, value_numeric
			FROM health_logs
			WHERE patient_id = ? AND metric_type = 'glucose' AND value_numeric IS NOT NULL
			ORDER BY measured_at::date DESC, measured_at DESC
		),
		bp AS (
			SELECT DISTINCT ON (measured_at::date) measured_at::date AS log_date, value_jsonb::text AS bp_raw
			FROM health_logs
			WHERE patient_id = ? AND metric_type = 'bp' AND value_jsonb IS NOT NULL
			ORDER BY measured_at::date DESC, measured_at DESC
		),
		wt AS (
			SELECT DISTINCT ON (measured_at::date) measured_at::date AS log_date, value_numeric
			FROM health_logs
			WHERE patient_id = ? AND metric_type = 'weight' AND value_numeric IS NOT NULL
			ORDER BY measured_at::date DESC, measured_at DESC
		),
		scores AS (
			SELECT DISTINCT ON (df.feature_date) df.feature_date AS log_date, rs.score
			FROM risk_scores rs
			JOIN daily_features df ON df.id = rs.daily_feature_id
			WHERE rs.patient_id = ?
			ORDER BY df.feature_date DESC, rs.scored_at DESC
		)
		SELECT
			d.log_date,
			g.value_numeric AS blood_sugar,
			b.bp_raw,
			w.value_numeric AS weight,
			s.score AS health_score
		FROM days d
		LEFT JOIN glucose g USING (log_date)
		LEFT JOIN bp b USING (log_date)
		LEFT JOIN wt w USING (log_date)
		LEFT JOIN scores s USING (log_date)
		ORDER BY d.log_date DESC
	`, patientID, limit, patientID, patientID, patientID, patientID).Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("getting record history: %w", err)
	}
	return rows, nil
}
