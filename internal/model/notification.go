package model

import "time"

// PatientNotificationResponse adalah satu item inbox in-app pasien. Bentuknya seragam untuk
// semua tipe (consultation_reply, daily_reminder): title/body sudah siap-tampil, sedangkan
// detail spesifik per-tipe ada di Data untuk keperluan deep-link di app.
type PatientNotificationResponse struct {
	ID        string                  `json:"id"`
	Type      string                  `json:"type"`
	Title     string                  `json:"title"`
	Body      string                  `json:"body"`
	IsRead    bool                    `json:"is_read"`
	ReadAt    *time.Time              `json:"read_at"`
	CreatedAt time.Time               `json:"created_at"`
	Data      PatientNotificationData `json:"data"`
}

// PatientNotificationData memuat field spesifik-tipe. Untuk consultation_reply berisi
// consultation_id + nakes_name; untuk daily_reminder keduanya null.
type PatientNotificationData struct {
	ConsultationID *string `json:"consultation_id"`
	NakesName      *string `json:"nakes_name"`
}

type UnreadCountResponse struct {
	UnreadCount int64 `json:"unread_count"`
}

type MarkAllReadResponse struct {
	UpdatedCount int64 `json:"updated_count"`
}
