// internal/entity/device_push_token.go
package entity

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Konstanta nilai kolom device_push_tokens.platform — hindari magic string (be_implementation §3).
const (
	DevicePlatformAndroid = "android"
	DevicePlatformIOS     = "ios"
)

// DevicePushToken memetakan tabel `device_push_tokens` — token FCM untuk push notification
// native Patient App. Multi-device: satu pasien boleh punya banyak baris aktif sekaligus.
// `IsActive` dipakai baik untuk soft-delete manual (logout) maupun auto-cleanup saat FCM
// menandai token invalid.
type DevicePushToken struct {
	ID        string    `gorm:"column:id;primaryKey"`
	PatientID string    `gorm:"column:patient_id"`
	Platform  string    `gorm:"column:platform"`
	Token     string    `gorm:"column:token"`
	IsActive  bool      `gorm:"column:is_active"`
	CreatedAt time.Time `gorm:"column:created_at"`
	UpdatedAt time.Time `gorm:"column:updated_at"`
}

func (DevicePushToken) TableName() string { return "device_push_tokens" }

func (d *DevicePushToken) BeforeCreate(tx *gorm.DB) error {
	if d.ID == "" {
		d.ID = uuid.New().String()
	}
	return nil
}
