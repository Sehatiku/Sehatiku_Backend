package entity

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Consultation struct {
	ID        string    `gorm:"column:id;primaryKey"`
	PatientID string    `gorm:"column:patient_id"`
	Complaint string    `gorm:"column:complaint"`
	Status    string    `gorm:"column:status"`
	CreatedAt time.Time `gorm:"column:created_at"`
}

func (Consultation) TableName() string { return "consultations" }

func (c *Consultation) BeforeCreate(tx *gorm.DB) error {
	if c.ID == "" {
		c.ID = uuid.New().String()
	}
	return nil
}
