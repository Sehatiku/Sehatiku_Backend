package repository

import (
	"fmt"
	"sehatiku-backend/internal/entity"

	"gorm.io/gorm"
)

type NakesRepository struct {
	Repository[entity.Nakes]
}

func (r *NakesRepository) FindByUsername(db *gorm.DB, username string) (*entity.Nakes, error) {
	var nakes entity.Nakes
	if err := db.Where("username = ?", username).First(&nakes).Error; err != nil {
		return nil, fmt.Errorf("finding nakes by username: %w", err)
	}
	return &nakes, nil
}

func (r *NakesRepository) FindByNIK(db *gorm.DB, nik string) (*entity.Nakes, error) {
	var nakes entity.Nakes
	if err := db.Where("nik = ?", nik).First(&nakes).Error; err != nil {
		return nil, fmt.Errorf("finding nakes by nik: %w", err)
	}
	return &nakes, nil
}

func (r *NakesRepository) FindByID(db *gorm.DB, id string) (*entity.Nakes, error) {
	var nakes entity.Nakes
	if err := db.Where("id = ?", id).First(&nakes).Error; err != nil {
		return nil, fmt.Errorf("finding nakes by id: %w", err)
	}
	return &nakes, nil
}

func (r *NakesRepository) FindByFaskesID(db *gorm.DB, faskesID string) ([]entity.Nakes, error) {
	var list []entity.Nakes
	if err := db.Where("faskes_id = ?", faskesID).Order("enrolled_at DESC").Find(&list).Error; err != nil {
		return nil, fmt.Errorf("finding nakes by faskes_id: %w", err)
	}
	return list, nil
}
