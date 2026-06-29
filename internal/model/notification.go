package model

import "time"

type PatientNotificationResponse struct {
	ID          string    `json:"id"`
	MessageType string    `json:"message_type"`
	Payload     string    `json:"payload"`
	CreatedAt   time.Time `json:"created_at"`
}
