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
