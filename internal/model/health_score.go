package model

import "time"

// HealthScoreResponse adalah hasil skoring harian pasien: health score (1..100) +
// status + faktor terbesar (top_penalties dari SHAP). Dipakai oleh
// GET /api/v1/patients/health-score.
type HealthScoreResponse struct {
	HealthScore  float64   `json:"health_score"`
	Status       string    `json:"status"`       // enum DB: aman / waswas / bahaya
	StatusLabel  string    `json:"status_label"` // tampilan: Sehat / Waswas / Parah
	Message      string    `json:"message"`
	TopPenalties []string  `json:"top_penalties"`
	ScoredAt     time.Time `json:"scored_at"`
}
