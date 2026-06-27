package model

type DashboardSummaryResponse struct {
	TotalPasien  int64 `json:"total_pasien"`
	RisikoBahaya int64 `json:"risiko_bahaya"`
	StatusAman   int64 `json:"status_aman"`
}

type PatientQueueItem struct {
	PatientID   string `json:"patient_id"`
	FullName    string `json:"full_name"`
	Age         int    `json:"age"`
	DiseaseType string `json:"disease_type"`
	RiskScore   int    `json:"risk_score"`
	RiskLabel   string `json:"risk_label"`
	Status      string `json:"status"`
	MainFactor  string `json:"main_factor"`
}

// RiskFactor is the shape of each element in risk_scores.top_factors JSONB.
type RiskFactor struct {
	Feature   string  `json:"feature"`
	ShapValue float64 `json:"shap_value"`
	Direction string  `json:"direction"`
}
