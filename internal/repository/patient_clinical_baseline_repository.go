package repository

import (
	"fmt"
	"sehatiku-backend/internal/entity"

	"gorm.io/gorm"
)

// PatientClinicalBaselineRepository reads clinical baselines for ML scoring.
type PatientClinicalBaselineRepository struct{}

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
