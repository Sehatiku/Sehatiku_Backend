package entity

import "time"

// PatientClinicalBaseline is a clinical baseline recorded per nakes visit. Supplies the
// baseline half of the ML payload (hba1c, egfr, bmi, systolic_bp). Clinical fields are
// nullable (a visit may not measure all of them).
type PatientClinicalBaseline struct {
	ID          string    `gorm:"column:id;primaryKey"`
	PatientID   string    `gorm:"column:patient_id"`
	RecordedAt  time.Time `gorm:"column:recorded_at"`
	HbA1c       *float64  `gorm:"column:hba1c"`
	EGFR        *float64  `gorm:"column:egfr"`
	BMI         *float64  `gorm:"column:bmi"`
	SystolicBP  *int      `gorm:"column:systolic_bp"`
	DiastolicBP *int      `gorm:"column:diastolic_bp"`
	CreatedAt   time.Time `gorm:"column:created_at"`
}

func (PatientClinicalBaseline) TableName() string {
	return "patient_clinical_baselines"
}
