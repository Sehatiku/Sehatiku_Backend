package entity

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// Nilai kolom escalations — sesuai enum DB (migration 000001/000005).
const (
	EscalationTierAcuteToday    = "acute_today"
	EscalationTierTrendThisWeek = "trend_this_week"

	EscalationStatusSent      = "sent"
	EscalationStatusViewed    = "viewed"
	EscalationStatusActed     = "acted"
	EscalationStatusDismissed = "dismissed"

	EscalationFeedbackAccurate   = "accurate"
	EscalationFeedbackInaccurate = "inaccurate"
)

// Escalation memetakan tabel `escalations` — peristiwa klinis (pasien berisiko) + feedback
// nakes. Mutable: status (sent→viewed→acted→dismissed) dan feedback berkembang setelah dibuat.
// Satu escalation bisa memicu beberapa baris `notifications` (transport) — lihat docs/erd.md.
type Escalation struct {
	ID              string     `gorm:"column:id;primaryKey"`
	PatientID       string     `gorm:"column:patient_id"`
	RiskScoreID     string     `gorm:"column:risk_score_id"`
	FaskesID        string     `gorm:"column:faskes_id"`
	AssignedNakesID string     `gorm:"column:assigned_nakes_id"`
	Tier            string     `gorm:"column:tier"`
	Channel         string     `gorm:"column:channel"`
	Status          string     `gorm:"column:status"`
	SentAt          time.Time  `gorm:"column:sent_at"`
	ViewedAt        *time.Time `gorm:"column:viewed_at"`
	ActedAt         *time.Time `gorm:"column:acted_at"`
	Feedback        *string    `gorm:"column:feedback"`
	FeedbackBy      *string    `gorm:"column:feedback_by"`
	FeedbackAt      *time.Time `gorm:"column:feedback_at"`
	CreatedAt       time.Time  `gorm:"column:created_at"`
	UpdatedAt       time.Time  `gorm:"column:updated_at"`
}

func (Escalation) TableName() string { return "escalations" }

func (e *Escalation) BeforeCreate(tx *gorm.DB) error {
	if e.ID == "" {
		e.ID = uuid.New().String()
	}
	return nil
}
