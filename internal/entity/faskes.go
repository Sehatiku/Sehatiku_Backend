package entity

import "time"

type Faskes struct {
	ID           string    `gorm:"column:id;primaryKey"`
	Name         string    `gorm:"column:name"`
	Type         string    `gorm:"column:type"`
	Address      string    `gorm:"column:address"`
	Region       string    `gorm:"column:region"`
	Status       string    `gorm:"column:status"`
	Username     string    `gorm:"column:username"`
	PasswordHash string    `gorm:"column:password_hash"`
	PhoneNumber  string    `gorm:"column:phone_number"`
	CreatedAt    time.Time `gorm:"column:created_at"`
	UpdatedAt    time.Time `gorm:"column:updated_at"`
}

func (Faskes) TableName() string {
	return "faskes"
}
