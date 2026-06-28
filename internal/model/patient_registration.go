package model

import "time"

// ── Patient Registration ─────────────────────────────────────────────────────

type PatientRegisterRequest struct {
	AssignedNakesID string `json:"assigned_nakes_id" validate:"required"`
	NIK             string `json:"nik"             validate:"required"`
	FullName        string `json:"full_name"       validate:"required"`
	DateOfBirth     string `json:"date_of_birth"   validate:"required"` // YYYY-MM-DD
	Sex             string `json:"sex"             validate:"required,oneof=male female"`
	Alamat          string `json:"alamat"          validate:"required"`
	PhoneNumber     string `json:"phone_number"    validate:"required"`
	CompanionName   string `json:"companion_name"  validate:"required"`
	CompanionPhone  string `json:"companion_phone" validate:"required"`
	DiseaseType     string `json:"disease_type"    validate:"required,oneof=diabetes_t2 hypertension both"`
	Username        string `json:"username"        validate:"required,min=4,max=50"`
	Password        string `json:"password"        validate:"required,min=8"`
}

type PatientRegisterResponse struct {
	PatientID   string    `json:"patient_id"`
	FaskesID    string    `json:"faskes_id"`
	FullName    string    `json:"full_name"`
	NIK         string    `json:"nik"`
	DiseaseType string    `json:"disease_type"`
	EnrolledAt  time.Time `json:"enrolled_at"`

	// Credentials dikembalikan SEKALI ke faskes saat registrasi supaya faskes punya
	// kanal cadangan menyampaikan login ke pasien/pendamping secara langsung bila
	// pengiriman WhatsApp gagal (mis. WhatsApp memblokir kontak baru — error 463).
	// Password yang sama persis dengan yang diinput faskes; tidak ada eksposur baru.
	Credentials PatientCredentials `json:"credentials"`

	// WADelivery melaporkan hasil pengiriman kredensial via WhatsApp per penerima
	// ("sent" / "failed"). Faskes memakai ini untuk memutuskan perlu menyampaikan
	// kredensial manual atau tidak.
	WADelivery WADeliveryStatus `json:"wa_delivery"`
}

type PatientCredentials struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type WADeliveryStatus struct {
	Patient   string `json:"patient"`             // "sent" | "failed"
	Companion string `json:"companion,omitempty"` // "sent" | "failed"; kosong bila tanpa pendamping
}
