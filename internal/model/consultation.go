package model

import "time"

type CreateConsultationRequest struct {
	Complaint string `json:"complaint" validate:"required,min=1,max=2000"`
}

type ConsultationResponse struct {
	ID        string    `json:"id"`
	PatientID string    `json:"patient_id"`
	Complaint string    `json:"complaint"`
	Status    string    `json:"status"`
	CreatedAt time.Time `json:"created_at"`
}
