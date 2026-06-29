package entity

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Notification memetakan tabel `notifications` — catatan transport pesan WA/SMS keluar
// (audit + dasar retry). Untuk credential_blast, payload TIDAK PERNAH menyimpan password
// (hanya metadata non-sensitif seperti username & nama penerima); password hanya hidup
// sebagai hash di tabel patients dan dikembalikan sekali ke faskes di response registrasi.
type Notification struct {
	ID                string     `gorm:"column:id;primaryKey"`
	PatientID         *string    `gorm:"column:patient_id"`
	NakesID           *string    `gorm:"column:nakes_id"`
	EscalationID      *string    `gorm:"column:escalation_id"`
	RecipientPhone    string     `gorm:"column:recipient_phone"`
	RecipientRole     string     `gorm:"column:recipient_role"`
	MessageType       string     `gorm:"column:message_type"`
	Channel           string     `gorm:"column:channel"`
	Payload           string     `gorm:"column:payload;type:jsonb"`
	Status            string     `gorm:"column:status"`
	ProviderMessageID *string    `gorm:"column:provider_message_id"`
	ErrorReason       *string    `gorm:"column:error_reason"`
	RetryCount        int        `gorm:"column:retry_count"`
	QueuedAt          time.Time  `gorm:"column:queued_at"`
	SentAt            *time.Time `gorm:"column:sent_at"`
	DeliveredAt       *time.Time `gorm:"column:delivered_at"`
	CreatedAt         time.Time  `gorm:"column:created_at"`
	UpdatedAt         time.Time  `gorm:"column:updated_at"`
}

// Konstanta nilai kolom notifications — hindari magic string tersebar (be_implementation §3).
const (
	NotificationStatusQueued = "queued"
	NotificationStatusSent   = "sent"
	NotificationStatusFailed = "failed"

	MessageTypeCredentialBlast  = "credential_blast"
	MessageTypeConsultationReply = "consultation_reply"

	NotificationChannelWhatsApp = "whatsapp"
	NotificationChannelInApp    = "in_app"

	RecipientRolePatient   = "patient"
	RecipientRoleCompanion = "companion"
	RecipientRoleNakes     = "nakes"
)

func (Notification) TableName() string {
	return "notifications"
}

func (n *Notification) BeforeCreate(tx *gorm.DB) error {
	if n.ID == "" {
		n.ID = uuid.New().String()
	}
	return nil
}
