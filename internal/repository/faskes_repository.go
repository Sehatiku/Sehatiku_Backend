package repository

import (
	"fmt"
	"sehatiku-backend/internal/entity"

	"gorm.io/gorm"
)

type FaskesRepository struct {
	Repository[entity.Faskes]
}

func (r *FaskesRepository) FindByUsername(db *gorm.DB, username string) (*entity.Faskes, error) {
	var faskes entity.Faskes
	if err := db.Where("username = ?", username).First(&faskes).Error; err != nil {
		return nil, fmt.Errorf("finding faskes by username: %w", err)
	}
	return &faskes, nil
}

func (r *FaskesRepository) FindByID(db *gorm.DB, id string) (*entity.Faskes, error) {
	var faskes entity.Faskes
	if err := db.Where("id = ?", id).First(&faskes).Error; err != nil {
		return nil, fmt.Errorf("finding faskes by id: %w", err)
	}
	return &faskes, nil
}
