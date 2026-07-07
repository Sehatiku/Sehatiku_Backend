package model

import "time"

// PreVisitBriefResponse adalah "dossier" satu pasien untuk dokter menjelang kontrol
// Prolanis: trajektori 30 hari, agregat, risiko terbaru, pola kepatuhan obat,
// eskalasi bulan berjalan, plus narasi klinis + draft pertanyaan anamnesis (Gemini).
// Selalu tersedia berapapun hari data pasien — Coverage memberi tahu kepadatannya.
type PreVisitBriefResponse struct {
	Period               SummaryPeriod     `json:"period"`
	Coverage             SummaryCoverage   `json:"coverage"`
	HistoryDays          int               `json:"history_days"`
	Trajectory           BriefTrajectory   `json:"trajectory"`
	Aggregates           *SummaryAggregates `json:"aggregates"`
	Risk                 *BriefRisk        `json:"risk"`
	MedAdherence         BriefMedAdherence `json:"med_adherence"`
	EscalationsThisMonth []BriefEscalation `json:"escalations_this_month"`
	Narrative            string            `json:"narrative"`
	AnamnesisQuestions   []string          `json:"anamnesis_questions"`
	GeneratedAt          time.Time         `json:"generated_at"`
}

// BriefTrajectory — series harian (kronologis naik) + slope tren per minggu.
// Slope nil bila titik data < 3 (tren belum bisa dihitung).
type BriefTrajectory struct {
	Daily                []RecordHistoryItem `json:"daily"`
	GlucoseSlopePerWeek  *float64            `json:"glucose_slope_per_week"`
	SystolicSlopePerWeek *float64            `json:"systolic_slope_per_week"`
}

// BriefRisk — risk score terbaru pasien. TopFactors berisi kalimat penalti dari ML
// (bukan objek SHAP — lihat NOTE di scoring_usecase.go), siap tampil apa adanya.
type BriefRisk struct {
	Score       int       `json:"score"`
	Status      string    `json:"status"`
	ScoringMode string    `json:"scoring_mode"`
	ScoredAt    time.Time `json:"scored_at"`
	TopFactors  []string  `json:"top_factors"`
}

// BriefMedAdherence — pola kepatuhan obat per hari dalam window: berapa hari minum,
// berapa hari terlewat, tanggal mana saja, dan hari-apa (Senin..Minggu) yang sering lupa.
type BriefMedAdherence struct {
	AdherenceRatePct float64        `json:"adherence_rate_pct"`
	TakenDays        int            `json:"taken_days"`
	MissedDays       int            `json:"missed_days"`
	MissedDates      []string       `json:"missed_dates"`
	MissedWeekdays   map[string]int `json:"missed_weekdays"`
}

// BriefEscalation — satu eskalasi bulan berjalan + hasilnya (status lifecycle & feedback nakes).
type BriefEscalation struct {
	Tier     string     `json:"tier"`
	Status   string     `json:"status"`
	Feedback *string    `json:"feedback"`
	SentAt   time.Time  `json:"sent_at"`
	ActedAt  *time.Time `json:"acted_at"`
}

// BriefReportData membungkus brief + identitas pasien untuk render laporan HTML
// (GET /nakes/patients/:id/brief/report). Brief-nya persis sama dengan endpoint JSON.
type BriefReportData struct {
	Patient BriefPatientHeader
	Brief   *PreVisitBriefResponse
}

// BriefPatientHeader — identitas pasien untuk kop laporan (tidak ada di PreVisitBriefResponse).
type BriefPatientHeader struct {
	FullName    string
	AgeYears    *int   // nil bila DateOfBirth tidak diketahui
	Sex         string // male|female
	DiseaseType string // diabetes_t2|hypertension|both
}
