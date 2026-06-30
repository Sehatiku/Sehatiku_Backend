package model

import "time"

// ── Patient Clinical Baseline (log/progress) ─────────────────────────────────
//
// Baseline pasien bersifat insert-only: tiap "update" oleh faskes = baris baseline
// baru (versi baru), bukan menimpa. "Baseline terkini" = baris terbaru; "progress" =
// daftar baris urut recorded_at menurun. Lihat docs/erd.md & docs/api_contract.md.

// CreateBaselineRequest adalah payload faskes untuk mencatat versi baseline baru.
// Field klinis di-reuse dari PatientBaselineRequest (sama dengan saat registrasi).
type CreateBaselineRequest struct {
	RecordedByNakesID string                 `json:"recorded_by_nakes_id" validate:"required"`
	RecordedAt        string                 `json:"recorded_at"` // opsional, YYYY-MM-DD; default waktu sekarang
	Notes             string                 `json:"notes"`
	Baseline          PatientBaselineRequest `json:"baseline" validate:"required"`
}

// BaselineDetailResponse memuat SELURUH field baseline + metadata audit. Dipakai untuk
// GET baseline terbaru (pre-fill form) dan sebagai response setelah POST baseline baru.
type BaselineDetailResponse struct {
	ID                  string    `json:"id"`
	PatientID           string    `json:"patient_id"`
	RecordedAt          time.Time `json:"recorded_at"`
	RecordedByNakesID   *string   `json:"recorded_by_nakes_id"`
	RecordedByNakesName string    `json:"recorded_by_nakes_name"`
	Notes               *string   `json:"notes"`

	// Demographics
	AgeYears int    `json:"age_years"`
	Sex      string `json:"sex"`

	// Anthropometry
	BMI                  float64 `json:"bmi"`
	BMICategory          string  `json:"bmi_category"`
	WaistCircumferenceCm float64 `json:"waist_circumference_cm"`
	CentralObesity       bool    `json:"central_obesity"`

	// Lifestyle
	SmokingStatus    string `json:"smoking_status"`
	AlcoholUse       bool   `json:"alcohol_use"`
	PhysicalActivity string `json:"physical_activity"`

	// Family history
	FamilyHistoryDiabetes bool `json:"family_history_diabetes"`
	FamilyHistoryCVD      bool `json:"family_history_cvd"`

	// Blood pressure
	SystolicBPMmhg     int    `json:"systolic_bp_mmhg"`
	DiastolicBPMmhg    int    `json:"diastolic_bp_mmhg"`
	HypertensionStatus string `json:"hypertension_status"`

	// Glucose / diabetes
	FastingGlucoseMgdl float64 `json:"fasting_glucose_mgdl"`
	HbA1cPct           float64 `json:"hba1c_pct"`
	DiabetesStatus     string  `json:"diabetes_status"`

	// Lipid panel
	TotalCholesterolMgdl float64 `json:"total_cholesterol_mgdl"`
	HDLMgdl              float64 `json:"hdl_mgdl"`
	LDLMgdl              float64 `json:"ldl_mgdl"`
	TriglyceidesMgdl     float64 `json:"triglycerides_mgdl"`

	// CVD risk
	CVDRisk10YrPct  float64 `json:"cvd_risk_10yr_pct"`
	CVDRiskCategory string  `json:"cvd_risk_category"`

	// Medications
	OnAntihypertensive bool `json:"on_antihypertensive"`
	OnAntidiabetic     bool `json:"on_antidiabetic"`
	OnStatin           bool `json:"on_statin"`

	// Risk target
	TargetRisk string `json:"target_risk"`

	// Kidney function
	EGFR float64 `json:"egfr"`
	UACR float64 `json:"uacr"`

	// ML cluster assignment (nullable)
	ClusterID        *int    `json:"cluster_id"`
	DiagnosisCluster *string `json:"diagnosis_cluster"`
	ClinicalGroup    *string `json:"clinical_group"`
}

// HealthScorePoint adalah satu titik tren health score (hasil skoring ML) pada waktu tertentu.
type HealthScorePoint struct {
	Score    int       `json:"score"`  // 0-100
	Status   string    `json:"status"` // aman | waswas | bahaya
	ScoredAt time.Time `json:"scored_at"`
}

// BaselineHistoryResponse menggabungkan progress baseline (paginated) dengan tren health
// score pasien sebagai dua deret terpisah. Dipakai endpoint baseline/history sisi faskes.
type BaselineHistoryResponse struct {
	BaselineHistory    []BaselineHistoryItem `json:"baseline_history"`
	HealthScoreHistory []HealthScorePoint    `json:"health_score_history"`
}

// BaselineHistoryItem adalah satu entri pada timeline progress baseline — hanya metrik
// kunci yang relevan untuk dipantau perubahannya.
type BaselineHistoryItem struct {
	ID                  string    `json:"id"`
	RecordedAt          time.Time `json:"recorded_at"`
	RecordedByNakesName string    `json:"recorded_by_nakes_name"`
	Notes               *string   `json:"notes"`

	BMI         float64 `json:"bmi"`
	BMICategory string  `json:"bmi_category"`

	SystolicBPMmhg     int    `json:"systolic_bp_mmhg"`
	DiastolicBPMmhg    int    `json:"diastolic_bp_mmhg"`
	HypertensionStatus string `json:"hypertension_status"`

	FastingGlucoseMgdl float64 `json:"fasting_glucose_mgdl"`
	HbA1cPct           float64 `json:"hba1c_pct"`
	DiabetesStatus     string  `json:"diabetes_status"`

	TotalCholesterolMgdl float64 `json:"total_cholesterol_mgdl"`
	HDLMgdl              float64 `json:"hdl_mgdl"`
	LDLMgdl              float64 `json:"ldl_mgdl"`
	TriglyceidesMgdl     float64 `json:"triglycerides_mgdl"`

	CVDRisk10YrPct  float64 `json:"cvd_risk_10yr_pct"`
	CVDRiskCategory string  `json:"cvd_risk_category"`

	EGFR float64 `json:"egfr"`
	UACR float64 `json:"uacr"`
}
