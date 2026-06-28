package model

import "time"

// CreateHealthLogRequest adalah body input data harian pasien (satu metrik per request).
// Field nilai bersifat polimorfik tergantung MetricType; validasi range/format per-metric
// dilakukan di usecase (struct tag tidak cukup untuk aturan polimorfik). measured_at
// dikirim client (RFC3339 / ISO 8601), bukan diisi server — lihat docs/api_guide.md §7.
type CreateHealthLogRequest struct {
	MetricType   string   `json:"metric_type" validate:"required,oneof=glucose bp med_adherence food activity sleep stress smoking alcohol weight"`
	ValueNumeric *float64 `json:"value_numeric"`
	ValueText    string   `json:"value_text"`
	Systolic     *int     `json:"systolic"`
	Diastolic    *int     `json:"diastolic"`
	MeasuredAt   string   `json:"measured_at" validate:"required"`
}

// BPValue adalah representasi tekanan darah pada response (dari value_jsonb).
type BPValue struct {
	Systolic  int `json:"systolic"`
	Diastolic int `json:"diastolic"`
}

// HealthLogResponse adalah representasi satu health log yang berhasil dicatat.
type HealthLogResponse struct {
	ID            string    `json:"id"`
	PatientID     string    `json:"patient_id"`
	MetricType    string    `json:"metric_type"`
	ValueNumeric  *float64  `json:"value_numeric,omitempty"`
	ValueText     string    `json:"value_text,omitempty"`
	BloodPressure *BPValue  `json:"blood_pressure,omitempty"`
	MeasuredAt    time.Time `json:"measured_at"`
	LoggedBy      string    `json:"logged_by"`
	Source        string    `json:"source"`
	CreatedAt     time.Time `json:"created_at"`
}
