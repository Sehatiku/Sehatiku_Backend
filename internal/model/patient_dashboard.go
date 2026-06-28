package model

import "time"

type PatientDashboardResponse struct {
	Profile            PatientDashboardProfile      `json:"profile"`
	Risk               PatientDashboardRisk         `json:"risk"`
	LatestMeasurements PatientDashboardMeasurements `json:"latest_measurements"`
	Logging            PatientDashboardLogging      `json:"logging"`
	Recommendations    []string                     `json:"recommendations"`
}

type PatientDashboardProfile struct {
	FullName          string `json:"full_name"`
	Age               int    `json:"age"`
	DiseaseType       string `json:"disease_type"`
	CompanionName     string `json:"companion_name"`
	CompanionPhone    string `json:"companion_phone"`
	AssignedNakesName string `json:"assigned_nakes_name"`
}

type PatientDashboardRisk struct {
	Score      int        `json:"score"`
	RiskLabel  string     `json:"risk_label"`
	Status     string     `json:"status"`
	MainFactor string     `json:"main_factor"`
	ScoredAt   *time.Time `json:"scored_at"`
}

type PatientDashboardMeasurements struct {
	Glucose       *GlucoseReading `json:"glucose"`
	BloodPressure *BPReading      `json:"blood_pressure"`
}

type GlucoseReading struct {
	Value      float64   `json:"value"`
	MeasuredAt time.Time `json:"measured_at"`
}

type BPReading struct {
	Systolic   int       `json:"systolic"`
	Diastolic  int       `json:"diastolic"`
	MeasuredAt time.Time `json:"measured_at"`
}

type PatientDashboardLogging struct {
	LoggedToday bool `json:"logged_today"`
	StreakDays  int  `json:"streak_days"`
}
