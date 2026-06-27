package model

import "time"

// ── Patient List (faskes view) ───────────────────────────────────────────────

type PatientListItem struct {
	PatientID      string    `json:"patient_id"`
	FullName       string    `json:"full_name"`
	NIK            string    `json:"nik"`
	Sex            string    `json:"sex"`
	Age            int       `json:"age"`
	DiseaseType    string    `json:"disease_type"`
	PhoneNumber    string    `json:"phone_number"`
	CompanionName  string    `json:"companion_name"`
	CompanionPhone string    `json:"companion_phone"`
	Status         string    `json:"status"`
	EnrolledAt     time.Time `json:"enrolled_at"`
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
