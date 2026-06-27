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

// FindByFaskesID mengembalikan satu halaman pasien milik faskes (semua status),
// diurutkan enrolled_at DESC, beserta total seluruh pasien faskes untuk pagination.
func (r *PatientRepository) FindByFaskesID(db *gorm.DB, faskesID string, limit, offset int) ([]entity.Patient, int64, error) {
	var total int64
	if err := db.Model(&entity.Patient{}).Where("faskes_id = ?", faskesID).Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("counting patients by faskes_id: %w", err)
	}

	var patients []entity.Patient
	if err := db.Where("faskes_id = ?", faskesID).
		Order("enrolled_at DESC").
		Limit(limit).Offset(offset).
		Find(&patients).Error; err != nil {
		return nil, 0, fmt.Errorf("finding patients by faskes_id: %w", err)
	}
	return patients, total, nil
}
