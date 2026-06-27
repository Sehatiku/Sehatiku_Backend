package model

import "time"

// ── Patient Registration ─────────────────────────────────────────────────────

type PatientRegisterRequest struct {
	AssignedNakesID string `json:"assigned_nakes_id" validate:"required"`
	NIK            string `json:"nik"             validate:"required"`
	FullName       string `json:"full_name"       validate:"required"`
	DateOfBirth    string `json:"date_of_birth"   validate:"required"` // YYYY-MM-DD
	Sex            string `json:"sex"             validate:"required,oneof=male female"`
	Alamat         string `json:"alamat"          validate:"required"`
	PhoneNumber    string `json:"phone_number"    validate:"required"`
	CompanionName  string `json:"companion_name"  validate:"required"`
	CompanionPhone string `json:"companion_phone" validate:"required"`
	DiseaseType    string `json:"disease_type"    validate:"required,oneof=diabetes_t2 hypertension both"`
	Username       string `json:"username"        validate:"required,min=4,max=50"`
	Password       string `json:"password"        validate:"required,min=8"`
}

type PatientRegisterResponse struct {
	PatientID   string    `json:"patient_id"`
	FaskesID    string    `json:"faskes_id"`
	FullName    string    `json:"full_name"`
	NIK         string    `json:"nik"`
	DiseaseType string    `json:"disease_type"`
	EnrolledAt  time.Time `json:"enrolled_at"`
}
