package repository

import (
	"fmt"
	"sehatiku-backend/internal/entity"

	"gorm.io/gorm"
)

// ConsultationRepository menangani penulisan ke tabel consultations.
type ConsultationRepository struct{}

func (r *ConsultationRepository) Create(db *gorm.DB, c *entity.Consultation) error {
	if err := db.Create(c).Error; err != nil {
		return fmt.Errorf("creating consultation: %w", err)
	}
	return nil
}
