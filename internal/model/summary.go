package model

import "time"

// SummaryResponse adalah ringkasan kesehatan pasien pada satu window (7/14/30 hari),
// gabungan angka agregat (dihitung backend dari health_logs) + narasi (Gemini).
//
// Bila window yang diminta belum cukup ditopang riwayat data pasien, Available=false,
// Narrative kosong, dan hanya AvailableWindows yang diisi — frontend memakai itu untuk
// menampilkan window mana saja yang valid.
type SummaryResponse struct {
	Window           int   `json:"window"`
	Available        bool  `json:"available"`
	AvailableWindows []int `json:"available_windows"`
	// HistoryDays = jumlah hari riwayat pencatatan pasien (log pertama s.d. hari ini, WIB).
	// Berguna untuk frontend menampilkan progres "X dari N hari" saat data belum cukup.
	HistoryDays int `json:"history_days"`
	// Message = penjelasan ramah ketika Available=false (data belum cukup). Kosong saat Available=true.
	Message     string             `json:"message,omitempty"`
	Period      *SummaryPeriod     `json:"period,omitempty"`
	Coverage    *SummaryCoverage   `json:"coverage,omitempty"`
	Aggregates  *SummaryAggregates `json:"aggregates,omitempty"`
	Risk        *SummaryRisk       `json:"risk,omitempty"`
	Narrative   string             `json:"narrative"`
	GeneratedAt time.Time          `json:"generated_at"`
}

type SummaryPeriod struct {
	Start string `json:"start"` // YYYY-MM-DD (WIB)
	End   string `json:"end"`   // YYYY-MM-DD (WIB)
}

type SummaryCoverage struct {
	LoggedDays int `json:"logged_days"`
	WindowDays int `json:"window_days"`
	StreakDays int `json:"streak_days"`
}

// SummaryAggregates — tiap sub-bagian nil jika tidak ada data metrik tsb di window.
type SummaryAggregates struct {
	Glucose       *GlucoseAggregate      `json:"glucose"`
	BloodPressure *BPAggregate           `json:"blood_pressure"`
	MedAdherence  *MedAdherenceAggregate `json:"med_adherence"`
	Nutrition     *NutritionAggregate    `json:"nutrition"`
	Activity      *ActivityAggregate     `json:"activity"`
	Sleep         *SleepAggregate        `json:"sleep"`
	Stress        *StressAggregate       `json:"stress"`
	Weight        *WeightAggregate       `json:"weight"`
}

type GlucoseAggregate struct {
	AvgMgDl float64 `json:"avg_mgdl"`
	MinMgDl float64 `json:"min_mgdl"`
	MaxMgDl float64 `json:"max_mgdl"`
	Count   int     `json:"count"`
}

type BPAggregate struct {
	AvgSystolic  float64 `json:"avg_systolic"`
	AvgDiastolic float64 `json:"avg_diastolic"`
	Count        int     `json:"count"`
}

type MedAdherenceAggregate struct {
	AdherenceRatePct float64 `json:"adherence_rate_pct"` // 0-100
	Count            int     `json:"count"`
}

type NutritionAggregate struct {
	AvgKcalPerDay     float64 `json:"avg_kcal_per_day"`
	AvgCarbsGPerDay   float64 `json:"avg_carbs_g_per_day"`
	AvgSodiumMgPerDay float64 `json:"avg_sodium_mg_per_day"`
	MealCount         int     `json:"meal_count"`
}

type ActivityAggregate struct {
	AvgMinutesPerDay float64 `json:"avg_minutes_per_day"`
	TotalMinutes     float64 `json:"total_minutes"`
	Count            int     `json:"count"`
}

type SleepAggregate struct {
	AvgHours float64 `json:"avg_hours"`
	Count    int     `json:"count"`
}

type StressAggregate struct {
	AvgLevel float64 `json:"avg_level"`
	Count    int     `json:"count"`
}

type WeightAggregate struct {
	StartKg  float64 `json:"start_kg"`
	LatestKg float64 `json:"latest_kg"`
	DeltaKg  float64 `json:"delta_kg"`
	Count    int     `json:"count"`
}

type SummaryRisk struct {
	Score    *int       `json:"score"`
	Status   string     `json:"status"`
	ScoredAt *time.Time `json:"scored_at"`
}
