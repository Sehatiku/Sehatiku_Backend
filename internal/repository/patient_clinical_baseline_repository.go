package repository

import (
	"fmt"
	"sehatiku-backend/internal/entity"

	"gorm.io/gorm"
)

// PatientClinicalBaselineRepository reads clinical baselines for ML scoring.
type PatientClinicalBaselineRepository struct{}

// Create inserts a new clinical baseline record with auto-generated UUID.
func (r *PatientClinicalBaselineRepository) Create(db *gorm.DB, baseline *entity.PatientClinicalBaseline) error {
	if err := db.Create(baseline).Error; err != nil {
		return fmt.Errorf("creating patient clinical baseline: %w", err)
	}
	return nil
}

// FindLatestByPatient returns the most recent baseline for a patient.
// Returns gorm.ErrRecordNotFound when the patient has no baseline yet.
func (r *PatientClinicalBaselineRepository) FindLatestByPatient(db *gorm.DB, patientID string) (*entity.PatientClinicalBaseline, error) {
	var baseline entity.PatientClinicalBaseline
	err := db.Where("patient_id = ?", patientID).
		Order("recorded_at DESC").
		First(&baseline).Error
	if err != nil {
		return nil, fmt.Errorf("finding latest clinical baseline: %w", err)
	}
	return &baseline, nil
}

// ListByPatient mengembalikan satu halaman baseline pasien, terbaru-dulu (recorded_at DESC),
// beserta total seluruh baseline pasien untuk pagination. Dipakai untuk menampilkan progress
// baseline sepanjang waktu.
func (r *PatientClinicalBaselineRepository) ListByPatient(db *gorm.DB, patientID string, limit, offset int) ([]entity.PatientClinicalBaseline, int64, error) {
	var total int64
	if err := db.Model(&entity.PatientClinicalBaseline{}).
		Where("patient_id = ?", patientID).
		Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("counting clinical baselines by patient: %w", err)
	}

	var baselines []entity.PatientClinicalBaseline
	if err := db.Where("patient_id = ?", patientID).
		Order("recorded_at DESC").
		Limit(limit).Offset(offset).
		Find(&baselines).Error; err != nil {
		return nil, 0, fmt.Errorf("listing clinical baselines by patient: %w", err)
	}
	return baselines, total, nil
}
