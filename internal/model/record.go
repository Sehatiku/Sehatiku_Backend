package model

import "time"

// CreateRecordRequest adalah body input catatan harian pasien (satu form, banyak metrik).
// Minimal satu field metrik harus diisi. Berbeda dari POST /health-logs yang satu request
// per metrik, endpoint ini menerima semua metrik sekaligus untuk UX form native.
type CreateRecordRequest struct {
	BloodSugar    *float64 `json:"blood_sugar"`
	Systolic      *int     `json:"systolic"`
	Diastolic     *int     `json:"diastolic"`
	Weight        *float64 `json:"weight"`
	MedicineTaken *bool    `json:"medicine_taken"`
	Meals         string   `json:"meals"`
	RecordedAt    string   `json:"recorded_at" validate:"required"`
}

type CreateRecordResponse struct {
	RecordedAt time.Time `json:"recorded_at"`
	Created    []string  `json:"created"`
}

type RecordHistoryItem struct {
	Date       string   `json:"date"`
	BloodSugar *float64 `json:"blood_sugar"`
	Systolic   *int     `json:"systolic"`
	Diastolic  *int     `json:"diastolic"`
	Weight     *float64 `json:"weight"`
}
