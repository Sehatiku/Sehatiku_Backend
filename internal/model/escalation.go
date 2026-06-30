package model

import "time"

// EscalationQueueItem adalah satu baris antrean eskalasi di dashboard nakes.
// patient_name & risk_score/risk_status di-JOIN-resolve (tidak disimpan di tabel escalations).
type EscalationQueueItem struct {
	ID          string     `json:"id"`
	PatientID   string     `json:"patient_id"`
	PatientName string     `json:"patient_name"`
	Tier        string     `json:"tier"`
	Status      string     `json:"status"`
	RiskScore   int        `json:"risk_score"`
	RiskStatus  string     `json:"risk_status"`
	SentAt      time.Time  `json:"sent_at"`
	ViewedAt    *time.Time `json:"viewed_at"`
	ActedAt     *time.Time `json:"acted_at"`
	CreatedAt   time.Time  `json:"created_at"`
}

// SetEscalationFeedbackRequest — body PATCH /nakes/escalations/{id}/feedback.
type SetEscalationFeedbackRequest struct {
	Feedback string `json:"feedback" validate:"required,oneof=accurate inaccurate"`
}
