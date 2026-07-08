package model

import "time"

// ── Patient Registration ─────────────────────────────────────────────────────

type PatientRegisterRequest struct {
	AssignedNakesID string                 `json:"assigned_nakes_id" validate:"required"`
	NIK             string                 `json:"nik"               validate:"required"`
	FullName        string                 `json:"full_name"         validate:"required"`
	DateOfBirth     string                 `json:"date_of_birth"     validate:"required"` // YYYY-MM-DD
	Sex             string                 `json:"sex"               validate:"required,oneof=male female"`
	Alamat          string                 `json:"alamat"            validate:"required"`
	PhoneNumber     string                 `json:"phone_number"      validate:"required"`
	CompanionName   string                 `json:"companion_name"    validate:"required"`
	CompanionPhone  string                 `json:"companion_phone"   validate:"required"`
	DiseaseType     string                 `json:"disease_type"      validate:"required,oneof=diabetes_t2 hypertension both"`
	Username        string                 `json:"username"          validate:"required,min=4,max=50"`
	Password        string                 `json:"password"          validate:"required,min=8"`
	Baseline        PatientBaselineRequest `json:"baseline"          validate:"required"`
}

// PatientBaselineRequest holds the clinical baseline the faskes actually inputs at
// registration (or via the baseline-ocr prefill). It carries ONLY raw measured/lab values
// plus two dropdowns; every categorical/derived field (bmi_category, central_obesity,
// hypertension_status, diabetes_status, cluster_id, diagnosis_cluster, clinical_group) is
// computed server-side (see internal/usecase/baseline_derivation.go), and age_years/sex are
// taken from the patient record — so the faskes never re-types data a computer can derive.
// Boolean fields use *bool so that false is distinguishable from absent.
type PatientBaselineRequest struct {
	// Anthropometry (bmi_category & central_obesity derived from these)
	BMI                  float64 `json:"bmi"                    validate:"required,min=5,max=100"`
	WaistCircumferenceCm float64 `json:"waist_circumference_cm" validate:"required,min=20,max=250"`

	// Lifestyle
	SmokingStatus    string `json:"smoking_status"    validate:"required,oneof=never former current"`
	AlcoholUse       *bool  `json:"alcohol_use"       validate:"required"`
	PhysicalActivity string `json:"physical_activity" validate:"required,oneof=sedentary light moderate active"`

	// Family history
	FamilyHistoryDiabetes *bool `json:"family_history_diabetes" validate:"required"`
	FamilyHistoryCVD      *bool `json:"family_history_cvd"      validate:"required"`

	// Blood pressure (hypertension_status derived from these)
	SystolicBPMmhg  int `json:"systolic_bp_mmhg"  validate:"required,min=40,max=300"`
	DiastolicBPMmhg int `json:"diastolic_bp_mmhg" validate:"required,min=20,max=200"`

	// Glucose / diabetes (diabetes_status derived from these)
	FastingGlucoseMgdl float64 `json:"fasting_glucose_mgdl" validate:"required,min=20,max=1000"`
	HbA1cPct           float64 `json:"hba1c_pct"            validate:"required,min=1,max=20"`

	// Lipid panel
	TotalCholesterolMgdl float64 `json:"total_cholesterol_mgdl" validate:"required,min=50,max=1000"`
	HDLMgdl              float64 `json:"hdl_mgdl"               validate:"required,min=5,max=200"`
	LDLMgdl              float64 `json:"ldl_mgdl"               validate:"required,min=5,max=600"`
	TriglyceidesMgdl     float64 `json:"triglycerides_mgdl"     validate:"required,min=10,max=5000"`

	// CVD risk — optional (isi bila laporan memuatnya; tidak diturunkan pakai formula)
	CVDRisk10YrPct  float64 `json:"cvd_risk_10yr_pct" validate:"omitempty,gte=0,max=100"`
	CVDRiskCategory string  `json:"cvd_risk_category" validate:"omitempty,oneof=low moderate high very_high"`

	// Medications
	OnAntihypertensive *bool `json:"on_antihypertensive" validate:"required"`
	OnAntidiabetic     *bool `json:"on_antidiabetic"     validate:"required"`
	OnStatin           *bool `json:"on_statin"           validate:"required"`

	// Risk target dropdown {Rendah, Menengah, Tinggi}. clinical_group is set = target_risk.
	TargetRisk string `json:"target_risk" validate:"required,oneof=low medium high"`

	// Kidney function
	EGFR float64 `json:"egfr" validate:"required,min=0,max=200"`
	UACR float64 `json:"uacr" validate:"omitempty,gte=0"`

	// Diagnosis dropdown {diabetes, hipertensi, komplikasi} — one input replacing cluster_id +
	// diagnosis_cluster. Optional: defaults from the patient's disease_type when absent.
	Diagnosis *string `json:"diagnosis" validate:"omitempty,oneof=diabetes hipertensi komplikasi"`
}

// BaselineOCRResponse adalah hasil scan dokumen template baseline. DiseaseType diturunkan
// dari dropdown Diagnosis yang terbaca di dokumen (lihat usecase.deriveDiseaseType) agar
// faskes juga bisa mem-prefill field disease_type di form registrasi, bukan cuma baseline.
// "" bila diagnosis tidak terbaca/tidak dikenali.
type BaselineOCRResponse struct {
	DiseaseType string                  `json:"disease_type"`
	Baseline    *PatientBaselineRequest `json:"baseline"`
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

	// *_direct_link adalah link wa.me yang menunjuk ke nomor PENERIMA sendiri, dengan teks
	// undangan aktivasi (sapaan + username + link warm-up bot) sudah terisi. Faskes klik link
	// → WhatsApp faskes langsung membuka chat ke pasien/pendamping/nakes, tinggal tekan kirim.
	// SENGAJA tanpa password (password tetap jalan bot→penerima setelah warm-up). Kosong/
	// dihilangkan bila bot belum dipasangkan atau penerima tidak ada (mis. tanpa pendamping).
	// Lihat helper.BuildDirectInviteLink.
	PatientDirectLink   string `json:"patient_direct_link,omitempty"`
	CompanionDirectLink string `json:"companion_direct_link,omitempty"`
	NakesDirectLink     string `json:"nakes_direct_link,omitempty"`

	Status string `json:"status"` // "pending" (menunggu penerima chat bot) | "unavailable" (bot belum dipasangkan)
}
