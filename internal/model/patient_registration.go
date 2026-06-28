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

	// Credentials dikembalikan SEKALI ke faskes saat registrasi sebagai kanal cadangan
	// TERJAMIN: faskes selalu bisa menyampaikan login ke pasien/pendamping secara langsung.
	// Password yang sama persis dengan yang diinput faskes; tidak ada eksposur baru.
	Credentials PatientCredentials `json:"credentials"`

	// WAWarmup berisi link wa.me first-contact untuk pasien & pendamping. WhatsApp memblokir
	// pesan keluar ke kontak baru (error 463), jadi backend TIDAK mengirim kredensial duluan;
	// penerima harus menghubungi bot lewat link ini, lalu bot otomatis membalas kredensial.
	WAWarmup WAWarmupStatus `json:"wa_warmup"`
}

type PatientCredentials struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

// WAWarmupStatus melaporkan alur warm-up WhatsApp ke faskes. Faskes menampilkan/meneruskan
// link ke penerima supaya mereka menghubungi bot lebih dulu.
type WAWarmupStatus struct {
	BotPhone      string `json:"bot_phone"`                // nomor bot; "" bila device WA belum dipasangkan
	PatientLink   string `json:"patient_link,omitempty"`   // link wa.me untuk pasien
	CompanionLink string `json:"companion_link,omitempty"` // link wa.me untuk pendamping; kosong bila tanpa pendamping
	NakesLink     string `json:"nakes_link,omitempty"`     // link wa.me untuk nakes (dipakai pada registrasi nakes)

	// *_message adalah teks SIAP-TEMPEL (faskes-mediated) berisi sapaan + username + link
	// aktivasi — faskes tinggal salin-bagikan ke penerima via kanal pribadi mereka. SENGAJA
	// tanpa password (password tetap jalan bot→penerima setelah warm-up). Kosong/dihilangkan
	// bila link terkait kosong (bot belum dipasangkan) atau penerima tidak ada (mis. tanpa
	// pendamping). Lihat helper.BuildWarmupShareMessage.
	PatientMessage   string `json:"patient_message,omitempty"`
	CompanionMessage string `json:"companion_message,omitempty"`
	NakesMessage     string `json:"nakes_message,omitempty"`

	Status string `json:"status"` // "pending" (menunggu penerima chat bot) | "unavailable" (bot belum dipasangkan)
}
