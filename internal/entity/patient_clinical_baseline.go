package entity

import (
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

type PatientClinicalBaseline struct {
	ID                   string    `gorm:"column:id;primaryKey"`
	PatientID            string    `gorm:"column:patient_id"`
	RecordedAt           time.Time `gorm:"column:recorded_at"`

	// Demographics
	AgeYears             int    `gorm:"column:age_years"`
	Sex                  string `gorm:"column:sex"`

	// Anthropometry
	BMI                  float64 `gorm:"column:bmi"`
	BMICategory          string  `gorm:"column:bmi_category"`
	WaistCircumferenceCm float64 `gorm:"column:waist_circumference_cm"`
	CentralObesity       bool    `gorm:"column:central_obesity"`

	// Lifestyle
	SmokingStatus   string `gorm:"column:smoking_status"`
	AlcoholUse      bool   `gorm:"column:alcohol_use"`
	PhysicalActivity string `gorm:"column:physical_activity"`

	// Family history
	FamilyHistoryDiabetes bool `gorm:"column:family_history_diabetes"`
	FamilyHistoryCVD      bool `gorm:"column:family_history_cvd"`

	// Blood pressure
	SystolicBPMmhg    int    `gorm:"column:systolic_bp_mmhg"`
	DiastolicBPMmhg   int    `gorm:"column:diastolic_bp_mmhg"`
	HypertensionStatus string `gorm:"column:hypertension_status"`

	// Glucose / diabetes
	FastingGlucoseMgdl float64 `gorm:"column:fasting_glucose_mgdl"`
	HbA1cPct           float64 `gorm:"column:hba1c_pct"`
	DiabetesStatus     string  `gorm:"column:diabetes_status"`

	// Lipid panel
	TotalCholesterolMgdl float64 `gorm:"column:total_cholesterol_mgdl"`
	HDLMgdl              float64 `gorm:"column:hdl_mgdl"`
	LDLMgdl              float64 `gorm:"column:ldl_mgdl"`
	TriglyceidesMgdl     float64 `gorm:"column:triglycerides_mgdl"`

	// CVD risk
	CVDRisk10YrPct  float64 `gorm:"column:cvd_risk_10yr_pct"`
	CVDRiskCategory string  `gorm:"column:cvd_risk_category"`

	// Medications
	OnAntihypertensive bool `gorm:"column:on_antihypertensive"`
	OnAntidiabetic     bool `gorm:"column:on_antidiabetic"`
	OnStatin           bool `gorm:"column:on_statin"`

	// Risk target
	TargetRisk string `gorm:"column:target_risk"`

	// Kidney function
	EGFR float64 `gorm:"column:egfr"`
	UACR float64 `gorm:"column:uacr"`

	// ML cluster assignment (nullable)
	ClusterID        *int    `gorm:"column:cluster_id"`
	DiagnosisCluster *string `gorm:"column:diagnosis_cluster"`
	ClinicalGroup    *string `gorm:"column:clinical_group"`

	CreatedAt time.Time `gorm:"column:created_at"`
}

func (PatientClinicalBaseline) TableName() string {
	return "patient_clinical_baselines"
}

func (p *PatientClinicalBaseline) BeforeCreate(tx *gorm.DB) error {
	if p.ID == "" {
		p.ID = uuid.New().String()
	}
	return nil
}
