package entity

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// PatientNotification memetakan tabel `patient_notifications` — inbox in-app yang dibaca
// Patient App. Berbeda dari `notifications` (log transport WA/SMS): tabel ini tidak punya
// status pengiriman/retry/provider id, melainkan state baca/belum-baca via ReadAt.
type PatientNotification struct {
	ID             string     `gorm:"column:id;primaryKey"`
	PatientID      string     `gorm:"column:patient_id"`
	Type           string     `gorm:"column:type"`
	Title          string     `gorm:"column:title"`
	Body           string     `gorm:"column:body"`
	Payload        *string    `gorm:"column:payload;type:jsonb"`
	ConsultationID *string    `gorm:"column:consultation_id"`
	ReadAt         *time.Time `gorm:"column:read_at"` // nil = belum dibaca
	CreatedAt      time.Time  `gorm:"column:created_at"`
}

// Konstanta nilai kolom patient_notifications — hindari magic string (be_implementation §3).
const (
	PatientNotifTypeConsultationReply = "consultation_reply"
	PatientNotifTypeDailyReminder     = "daily_reminder"
	PatientNotifTypeEscalation        = "escalation"
)

func (PatientNotification) TableName() string {
	return "patient_notifications"
}

func (n *PatientNotification) BeforeCreate(tx *gorm.DB) error {
	if n.ID == "" {
		n.ID = uuid.New().String()
	}
	return nil
}
