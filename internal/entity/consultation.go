package entity

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

const (
	ConsultationStatusOpen    = "open"
	ConsultationStatusReplied = "replied"
)

type Consultation struct {
	ID                string     `gorm:"column:id;primaryKey"`
	PatientID         string     `gorm:"column:patient_id"`
	ComplaintSince    string     `gorm:"column:complaint_since"`
	ComplaintType     string     `gorm:"column:complaint_type"`
	ComplaintDetail   string     `gorm:"column:complaint_detail"`
	Status            string     `gorm:"column:status"`
	NakesNote         *string    `gorm:"column:nakes_note"`
	RepliedByNakesID  *string    `gorm:"column:replied_by_nakes_id"`
	RepliedAt         *time.Time `gorm:"column:replied_at"`
	CreatedAt         time.Time  `gorm:"column:created_at"`
	UpdatedAt         time.Time  `gorm:"column:updated_at"`
}

func (Consultation) TableName() string { return "consultations" }

func (c *Consultation) BeforeCreate(tx *gorm.DB) error {
	if c.ID == "" {
		c.ID = uuid.New().String()
	}
	return nil
}
