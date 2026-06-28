package repository

import (
	"fmt"
	"sehatiku-backend/internal/entity"

	"gorm.io/gorm"
)

// RiskScoreRepository writes ML scoring results to risk_scores (insert-only audit row).
type RiskScoreRepository struct{}

func (r *RiskScoreRepository) Create(db *gorm.DB, score *entity.RiskScore) error {
	if err := db.Create(score).Error; err != nil {
		return fmt.Errorf("creating risk score: %w", err)
	}
	return nil
}
