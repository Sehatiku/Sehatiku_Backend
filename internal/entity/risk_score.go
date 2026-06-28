package entity

import (
	"encoding/json"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type RiskScore struct {
	ID             string          `gorm:"column:id;primaryKey"`
	PatientID      string          `gorm:"column:patient_id"`
	DailyFeatureID string          `gorm:"column:daily_feature_id"`
	ModelVersionID *string         `gorm:"column:model_version_id"`
	Score          int             `gorm:"column:score"`
	Status         string          `gorm:"column:status"`
	ScoringMode    string          `gorm:"column:scoring_mode"`
	TopFactors     json.RawMessage `gorm:"column:top_factors;type:jsonb"`
	TriggeredRule  *string         `gorm:"column:triggered_rule"`
	ScoredAt       time.Time       `gorm:"column:scored_at"`
	CreatedAt      time.Time       `gorm:"column:created_at"`
}

func (RiskScore) TableName() string {
	return "risk_scores"
}

func (r *RiskScore) BeforeCreate(tx *gorm.DB) error {
	if r.ID == "" {
		r.ID = uuid.New().String()
	}
	return nil
}
