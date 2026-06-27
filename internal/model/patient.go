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
