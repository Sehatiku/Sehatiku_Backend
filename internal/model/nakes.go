package model

import "time"

// ── KTP OCR pre-fill ─────────────────────────────────────────────────────────

type KTPOCRResponse struct {
	NIK         string `json:"nik"`
	FullName    string `json:"full_name"`
	DateOfBirth string `json:"date_of_birth"` // YYYY-MM-DD
	Sex         string `json:"sex"`           // male | female
	Alamat      string `json:"alamat"`
}

// ── Nakes Registration ───────────────────────────────────────────────────────

type NakesRegisterRequest struct {
	NIK            string          `json:"nik"             validate:"required"`
	FullName       string          `json:"full_name"       validate:"required"`
	Alamat         string          `json:"alamat"          validate:"required"`
	PhoneNumber    string          `json:"phone_number"    validate:"required"`
	Role           string          `json:"role"            validate:"required,oneof=dokter kader admin"`
	Username       string          `json:"username"        validate:"required,min=4,max=50"`
	Password       string          `json:"password"        validate:"required,min=8"`
	Specialization *string         `json:"specialization"`
	Schedule       []ScheduleEntry `json:"schedule"`
}

type NakesRegisterResponse struct {
	NakesID    string    `json:"nakes_id"`
	FaskesID   string    `json:"faskes_id"`
	FullName   string    `json:"full_name"`
	Role       string    `json:"role"`
	NIK        string    `json:"nik"`
	EnrolledAt time.Time `json:"enrolled_at"`

	// Credentials dikembalikan SEKALI ke faskes sebagai kanal cadangan terjamin (sama
	// seperti registrasi pasien) — faskes selalu bisa menyampaikan login langsung ke nakes.
	Credentials NakesCredentials `json:"credentials"`

	// WAWarmup berisi link wa.me first-contact untuk nakes. Backend tidak mengirim kredensial
	// duluan (WhatsApp memblokir kontak baru, error 463); nakes menghubungi bot lewat link
	// ini, lalu bot otomatis membalas kredensial.
	WAWarmup WAWarmupStatus `json:"wa_warmup"`
}

type NakesCredentials struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// ── Nakes List ───────────────────────────────────────────────────────────────

type NakesListItem struct {
	NakesID     string    `json:"nakes_id"`
	FullName    string    `json:"full_name"`
	Role        string    `json:"role"`
	Username    string    `json:"username"`
	PhoneNumber string    `json:"phone_number"`
	Status      string    `json:"status"`
	EnrolledAt  time.Time `json:"enrolled_at"`
}

// ── Nakes Detail (faskes view) ───────────────────────────────────────────────

type NakesDetailResponse struct {
	NakesID     string    `json:"nakes_id"`
	FaskesID    string    `json:"faskes_id"`
	FullName    string    `json:"full_name"`
	Role        string    `json:"role"`
	NIK         string    `json:"nik"`
	Alamat      string    `json:"alamat"`
	PhoneNumber string    `json:"phone_number"`
	Username    string    `json:"username"`
	Status      string    `json:"status"`
	EnrolledAt  time.Time `json:"enrolled_at"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// ── Nakes Status Update ──────────────────────────────────────────────────────

type UpdateNakesStatusRequest struct {
	Status string `json:"status" validate:"required,oneof=active inactive"`
}

type UpdateNakesStatusResponse struct {
	NakesID  string `json:"nakes_id"`
	FullName string `json:"full_name"`
	Status   string `json:"status"`
}
