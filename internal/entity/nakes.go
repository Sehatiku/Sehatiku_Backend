package entity

import "time"

type Nakes struct {
	ID           string    `gorm:"column:id;primaryKey"`
	FaskesID     string    `gorm:"column:faskes_id"`
	Username     string    `gorm:"column:username"`
	PasswordHash string    `gorm:"column:password_hash"`
	FullName     string    `gorm:"column:full_name"`
	Role         string    `gorm:"column:role"`
	NIK          string    `gorm:"column:nik"`
	Alamat       string    `gorm:"column:alamat"`
	PhoneNumber  string    `gorm:"column:phone_number"`
	Status       string    `gorm:"column:status"`
	EnrolledAt   time.Time `gorm:"column:enrolled_at"`
	CreatedAt    time.Time `gorm:"column:created_at"`
	UpdatedAt    time.Time `gorm:"column:updated_at"`
}

func (Nakes) TableName() string {
	return "nakes"
}
