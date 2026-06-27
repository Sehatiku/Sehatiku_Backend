package entity

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type Patient struct {
	ID              string    `gorm:"column:id;primaryKey"`
	FaskesID        string    `gorm:"column:faskes_id"`
	AssignedNakesID string    `gorm:"column:assigned_nakes_id"`
	Username        string    `gorm:"column:username"`
	PasswordHash    string    `gorm:"column:password_hash"`
	FullName        string    `gorm:"column:full_name"`
	NIK             string    `gorm:"column:nik"`
	Alamat          string    `gorm:"column:alamat"`
	PhoneNumber     string    `gorm:"column:phone_number"`
	CompanionName   string    `gorm:"column:companion_name"`
	CompanionPhone  string    `gorm:"column:companion_phone"`
	DateOfBirth     *time.Time `gorm:"column:date_of_birth"`
	Sex             string    `gorm:"column:sex"`
	DiseaseType     string    `gorm:"column:disease_type"`
	Status          string    `gorm:"column:status"`
	EnrolledAt      time.Time `gorm:"column:enrolled_at"`
	CreatedAt       time.Time `gorm:"column:created_at"`
	UpdatedAt       time.Time `gorm:"column:updated_at"`
}

func (Patient) TableName() string {
	return "patients"
}

func (p *Patient) BeforeCreate(tx *gorm.DB) error {
	if p.ID == "" {
		p.ID = uuid.New().String()
	}
	return nil
}
