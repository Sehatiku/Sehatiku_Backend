package repository

import (
	"fmt"
	"sehatiku-backend/internal/entity"

	"gorm.io/gorm"
)

// HealthLogRepository menangani penulisan ke tabel health_logs.
// Tabel ini insert-only (full audit trail data medis, lihat docs/erd.md), karena itu
// repository ini SENGAJA tidak meng-embed Repository[T] generic — tidak ada Update/Delete
// yang boleh menyentuh baris health_logs.
type HealthLogRepository struct{}

func (r *HealthLogRepository) Create(db *gorm.DB, log *entity.HealthLog) error {
	if err := db.Create(log).Error; err != nil {
		return fmt.Errorf("creating health log: %w", err)
	}
	return nil
}

func (r *HealthLogRepository) FindByID(db *gorm.DB, id string) (*entity.HealthLog, error) {
	var log entity.HealthLog
	if err := db.Where("id = ?", id).First(&log).Error; err != nil {
		return nil, fmt.Errorf("finding health log by id: %w", err)
	}
	return &log, nil
}

// HasLogToday melaporkan apakah pasien sudah punya health_log HARI INI (zona WIB/
// Asia/Jakarta, konsisten dengan logged_today dashboard & HasExtremeReadingToday).
// Dipakai jalur WA untuk memberi konteks "sudah mencatat hari ini" saat pasien
// meminta template lagi — tidak memblokir input, hanya menandai.
func (r *HealthLogRepository) HasLogToday(db *gorm.DB, patientID string) (bool, error) {
	var count int64
	err := db.Model(&entity.HealthLog{}).
		Where(`patient_id = ?
			AND (measured_at AT TIME ZONE 'Asia/Jakarta')::date = (now() AT TIME ZONE 'Asia/Jakarta')::date`,
			patientID).
		Count(&count).Error
	if err != nil {
		return false, fmt.Errorf("checking today's log for patient %s: %w", patientID, err)
	}
	return count > 0, nil
}

// HasExtremeReadingToday melaporkan apakah pasien punya pembacaan ekstrem HARI INI (WIB):
// gula >= glucoseHigh atau <= glucoseLow, atau tensi sistolik >= systolicHigh / diastolik
// >= diastolicHigh. Dipakai sebagai pemicu eskalasi acute selain transisi status ke bahaya.
func (r *HealthLogRepository) HasExtremeReadingToday(db *gorm.DB, patientID string, glucoseHigh, glucoseLow, systolicHigh, diastolicHigh float64) (bool, error) {
	var count int64
	err := db.Model(&entity.HealthLog{}).
		Where(`patient_id = ?
			AND (measured_at AT TIME ZONE 'Asia/Jakarta')::date = (now() AT TIME ZONE 'Asia/Jakarta')::date
			AND (
				(metric_type = 'glucose' AND value_numeric IS NOT NULL AND (value_numeric >= ? OR value_numeric <= ?))
				OR (metric_type = 'bp' AND value_jsonb IS NOT NULL AND (
					(value_jsonb->>'systolic')::numeric >= ? OR (value_jsonb->>'diastolic')::numeric >= ?))
			)`, patientID, glucoseHigh, glucoseLow, systolicHigh, diastolicHigh).
		Count(&count).Error
	if err != nil {
		return false, fmt.Errorf("checking extreme reading for patient %s: %w", patientID, err)
	}
	return count > 0, nil
}
