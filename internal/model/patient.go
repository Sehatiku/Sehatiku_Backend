package model

import (
	"encoding/json"
	"time"
)

// ── Patient List (faskes view) ───────────────────────────────────────────────

type PatientListItem struct {
	PatientID      string           `json:"patient_id"`
	FullName       string           `json:"full_name"`
	NIK            string           `json:"nik"`
	Sex            string           `json:"sex"`
	Age            int              `json:"age"`
	DiseaseType    string           `json:"disease_type"`
	PhoneNumber    string           `json:"phone_number"`
	CompanionName  string           `json:"companion_name"`
	CompanionPhone string           `json:"companion_phone"`
	Status         string           `json:"status"`
	EnrolledAt     time.Time        `json:"enrolled_at"`
	// Risk score terbaru pasien — nil jika pasien belum pernah di-score
	HealthScore    *int             `json:"health_score"`    // 0-100, dari risk_scores.score
	RiskStatus     *string          `json:"risk_status"`     // aman | waswas | bahaya
	TopFactors     json.RawMessage  `json:"top_factors"`     // [{feature, shap_value, direction}] atau null
}

// ── Patient Detail (faskes view) ─────────────────────────────────────────────

type PatientDetailResponse struct {
	PatientID         string    `json:"patient_id"`
	FaskesID          string    `json:"faskes_id"`
	AssignedNakesID   string    `json:"assigned_nakes_id"`
	AssignedNakesName string    `json:"assigned_nakes_name"`
	FullName          string    `json:"full_name"`
	NIK               string    `json:"nik"`
	DateOfBirth       string    `json:"date_of_birth"` // YYYY-MM-DD, "" jika kosong
	Sex               string    `json:"sex"`
	Age               int       `json:"age"`
	Alamat            string    `json:"alamat"`
	PhoneNumber       string    `json:"phone_number"`
	CompanionName     string    `json:"companion_name"`
	CompanionPhone    string    `json:"companion_phone"`
	DiseaseType       string    `json:"disease_type"`
	Username          string    `json:"username"`
	Status            string    `json:"status"`
	EnrolledAt        time.Time `json:"enrolled_at"`
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`
}

// ── Patient Detail (nakes view) ──────────────────────────────────────────────

type NakesPatientDetailResponse struct {
	PatientDetail      PatientDetailResponse    `json:"patient_detail"`
	Baseline           *BaselineDetailResponse  `json:"baseline"`
	DailyLogs          []RecordHistoryItem      `json:"daily_logs"`
	Risk               *PatientRiskFactorStatus `json:"risk"`
	HealthScoreHistory []HealthScorePoint       `json:"health_score_history"`
}

type PatientRiskFactorStatus struct {
	Score       int             `json:"score"`
	Status      string          `json:"status"`
	ScoringMode string          `json:"scoring_mode"`
	TopFactors  json.RawMessage `json:"top_factors"`
}
