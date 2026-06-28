package entity

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// DailyFeature is one day's rolling-7 feature row for a patient — the exact 8 inputs
// the ML XGBoost model consumes. Computed from health_logs (see
// DailyFeatureRepository.ComputeRoll7) and passed to the ML /predict_health_score
// endpoint. The optional dashboard columns in the schema are omitted here.
type DailyFeature struct {
	ID          string    `gorm:"column:id;primaryKey"`
	PatientID   string    `gorm:"column:patient_id"`
	FeatureDate time.Time `gorm:"column:feature_date"`

	GlucoseMeanRoll7 float64 `gorm:"column:glucose_mean_roll7"`
	GlucoseCVRoll7   float64 `gorm:"column:glucose_cv_roll7"`
	SystolicRoll7    float64 `gorm:"column:systolic_roll7"`
	SodiumRoll7      float64 `gorm:"column:sodium_roll7"`
	SleepRoll7       float64 `gorm:"column:sleep_roll7"`
	ActivityPctRoll7 float64 `gorm:"column:activity_pct_roll7"`
	StressRoll7      float64 `gorm:"column:stress_roll7"`
	CarbsRoll7       float64 `gorm:"column:carbs_roll7"`

	CreatedAt time.Time `gorm:"column:created_at"`
}

func (DailyFeature) TableName() string {
	return "daily_features"
}

func (d *DailyFeature) BeforeCreate(tx *gorm.DB) error {
	if d.ID == "" {
		d.ID = uuid.New().String()
	}
	return nil
}
