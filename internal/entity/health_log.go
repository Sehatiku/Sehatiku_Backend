package entity

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// HealthLog adalah satu pengukuran kesehatan harian pasien (event stream insert-only).
// Satu baris = satu pengukuran. measured_at menyimpan waktu pengukuran asli (client-supplied),
// bukan waktu insert. Tipe nilai fleksibel sesuai metric_type:
//   - glucose / metrik numerik lain -> ValueNumeric
//   - bp (tekanan darah)            -> ValueJSONB {"systolic": N, "diastolic": N}
//   - food                          -> ValueText
//
// Lihat docs/erd.md (konvensi health_logs) dan db/migration/000003_raw_data.up.sql.
type HealthLog struct {
	ID           string    `gorm:"column:id;primaryKey"`
	PatientID    string    `gorm:"column:patient_id"`
	LoggedBy     string    `gorm:"column:logged_by"`
	MetricType   string    `gorm:"column:metric_type"`
	ValueNumeric *float64  `gorm:"column:value_numeric"`
	ValueText    *string   `gorm:"column:value_text"`
	ValueJSONB   *string   `gorm:"column:value_jsonb;type:jsonb"`
	MeasuredAt   time.Time `gorm:"column:measured_at"`
	Source       string    `gorm:"column:source"`
	CreatedAt    time.Time `gorm:"column:created_at"`
}

func (HealthLog) TableName() string {
	return "health_logs"
}

func (h *HealthLog) BeforeCreate(tx *gorm.DB) error {
	if h.ID == "" {
		h.ID = uuid.New().String()
	}
	return nil
}
