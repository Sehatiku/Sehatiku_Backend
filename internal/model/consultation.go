package model

import "time"

// ── Patient side ──────────────────────────────────────────────────────────────

type CreateConsultationRequest struct {
	ComplaintSince  string `json:"complaint_since"  validate:"required,min=1,max=500"`
	ComplaintType   string `json:"complaint_type"   validate:"required,min=1,max=500"`
	ComplaintDetail string `json:"complaint_detail" validate:"required,min=1,max=2000"`
}

type ConsultationResponse struct {
	ID              string     `json:"id"`
	PatientID       string     `json:"patient_id"`
	ComplaintSince  string     `json:"complaint_since"`
	ComplaintType   string     `json:"complaint_type"`
	ComplaintDetail string     `json:"complaint_detail"`
	Status          string     `json:"status"`
	NakesNote       *string    `json:"nakes_note"`
	RepliedAt       *time.Time `json:"replied_at"`
	CreatedAt       time.Time  `json:"created_at"`
}

// ── Nakes side ────────────────────────────────────────────────────────────────

// NakesConsultationItem is one row in the nakes consultation list.
// PatientName is JOIN-resolved — not stored in consultations table.
type NakesConsultationItem struct {
	ID              string     `json:"id"`
	PatientID       string     `json:"patient_id"`
	PatientName     string     `json:"patient_name"`
	ComplaintSince  string     `json:"complaint_since"`
	ComplaintType   string     `json:"complaint_type"`
	ComplaintDetail string     `json:"complaint_detail"`
	Status          string     `json:"status"`
	NakesNote       *string    `json:"nakes_note"`
	RepliedAt       *time.Time `json:"replied_at"`
	CreatedAt       time.Time  `json:"created_at"`
}

type ReplyConsultationRequest struct {
	NakesNote string `json:"nakes_note" validate:"required,min=1,max=2000"`
}
