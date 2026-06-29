package model

import "time"

type PatientNotificationResponse struct {
	ID             string    `json:"id"`
	MessageType    string    `json:"message_type"`
	NakesName      string    `json:"nakes_name"`
	NakesNote      string    `json:"nakes_note"`
	ConsultationID string    `json:"consultation_id"`
	CreatedAt      time.Time `json:"created_at"`
}
