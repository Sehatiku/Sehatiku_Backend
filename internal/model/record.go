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
	// Score dihitung setelah catatan tersimpan (roll-7 + ML). Bersifat best-effort:
	// di-omit (null) bila pasien belum punya baseline klinis atau ML sedang tak
	// terjangkau — catatan harian tetap tersimpan dan response tetap 201.
	Score *HealthScoreResponse `json:"score,omitempty"`
}

// TodayStatusResponse menjawab "apakah pasien sudah mengisi data harian hari ini?".
// Dipakai mobile untuk memunculkan pop-up pengingat kalau logged_today == false,
// sekaligus memberi tahu sudah berapa hari pasien tidak mengisi.
type TodayStatusResponse struct {
	LoggedToday      bool       `json:"logged_today"`
	DaysSinceLastLog *int       `json:"days_since_last_log"`
	LastLoggedAt     *time.Time `json:"last_logged_at"`
	Date             string     `json:"date"`
}

type RecordHistoryItem struct {
	Date        string   `json:"date"`
	BloodSugar  *float64 `json:"blood_sugar"`
	Systolic    *int     `json:"systolic"`
	Diastolic   *int     `json:"diastolic"`
	Weight      *float64 `json:"weight"`
	HealthScore *int     `json:"health_score"`
}
