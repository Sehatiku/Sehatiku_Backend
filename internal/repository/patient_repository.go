package repository

import (
	"fmt"
	"sehatiku-backend/internal/entity"

	"gorm.io/gorm"
)

type PatientRepository struct {
	Repository[entity.Patient]
}

func (r *PatientRepository) FindByUsername(db *gorm.DB, username string) (*entity.Patient, error) {
	var patient entity.Patient
	if err := db.Where("username = ?", username).First(&patient).Error; err != nil {
		return nil, fmt.Errorf("finding patient by username: %w", err)
	}
	return &patient, nil
}

func (r *PatientRepository) FindByNIK(db *gorm.DB, nik string) (*entity.Patient, error) {
	var patient entity.Patient
	if err := db.Where("nik = ?", nik).First(&patient).Error; err != nil {
		return nil, fmt.Errorf("finding patient by nik: %w", err)
	}
	return &patient, nil
}
