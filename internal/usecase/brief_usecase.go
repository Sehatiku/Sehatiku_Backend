package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"sehatiku-backend/internal/entity"
	"sehatiku-backend/internal/model"
	"sehatiku-backend/internal/repository"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// Pre-Visit Prolanis Brief — GET /api/v1/nakes/patients/:id/brief.
// "Dossier" 30 hari satu pasien untuk dokter menjelang kontrol bulanan, disusun
// on-demand (tanpa scheduler/jadwal kontrol — keputusan produk) dan di-cache 24 jam.
// Method ini hidup di SummaryUseCase karena berbagi seluruh dependensi summary
// (aggregates, Gemini, Redis, tenancy) + tiga repo tambahan di bawah.

const (
	briefWindowDays        = 30
	briefFallbackNarrative = "Ringkasan otomatis sedang tidak tersedia. Silakan tinjau angka trajektori, kepatuhan obat, dan eskalasi di bawah."
)

// Dependensi tambahan khusus brief — narrow interface, konvensi codebase.

type briefMedRepo interface {
	GetMedAdherenceDays(db *gorm.DB, patientID string, since time.Time) ([]repository.MedDayRaw, error)
}

type briefHistoryRepo interface {
	GetRecordHistory(db *gorm.DB, patientID string, limit int) ([]repository.RecordHistoryRaw, error)
}

type briefRiskRepo interface {
	FindLatestByPatient(db *gorm.DB, patientID string) (*entity.RiskScore, error)
}

type briefEscalationRepo interface {
	FindByPatientSince(db *gorm.DB, patientID string, since time.Time) ([]repository.EscalationBriefRow, error)
}

// GetNakesPatientBriefReport mengambil brief + identitas pasien untuk render laporan HTML.
// Reuse penuh GetNakesPatientBrief (cache Redis 24 jam + narasi Gemini) — hanya menambah
// kop pasien untuk laporan.
func (u *SummaryUseCase) GetNakesPatientBriefReport(ctx context.Context, faskesID, patientID string) (*model.BriefReportData, error) {
	// ponytail: double patient fetch (di sini + di dalam brief) — dua read terindeks yang
	// murah, tidak sepadan dengan menembuskan entity lewat signature brief.
	patient, err := u.PatientRepo.FindByID(u.DB, patientID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrPatientNotFound
		}
		return nil, fmt.Errorf("finding patient %s: %w", patientID, err)
	}
	if patient.FaskesID != faskesID {
		return nil, ErrPatientNotFound
	}

	brief, err := u.GetNakesPatientBrief(ctx, faskesID, patientID)
	if err != nil {
		return nil, err
	}

	return &model.BriefReportData{
		Patient: model.BriefPatientHeader{
			FullName:    patient.FullName,
			AgeYears:    ageYears(patient.DateOfBirth),
			Sex:         patient.Sex,
			DiseaseType: patient.DiseaseType,
		},
		Brief: brief,
	}, nil
}

// ageYears menghitung umur (tahun penuh) dari tanggal lahir, nil bila tidak diketahui.
func ageYears(dob *time.Time) *int {
	if dob == nil {
		return nil
	}
	now := time.Now().In(wibLocation)
	y := now.Year() - dob.Year()
	if now.YearDay() < dob.YearDay() {
		y--
	}
	if y < 0 {
		return nil
	}
	return &y
}

// GetNakesPatientBrief menyusun Pre-Visit Brief satu pasien untuk nakes. Tenancy:
// pasien milik faskes lain dikembalikan sebagai not-found (pola GetNakesPatientSummary).
func (u *SummaryUseCase) GetNakesPatientBrief(ctx context.Context, faskesID, patientID string) (*model.PreVisitBriefResponse, error) {
	patient, err := u.PatientRepo.FindByID(u.DB, patientID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrPatientNotFound
		}
		return nil, fmt.Errorf("finding patient %s: %w", patientID, err)
	}
	if patient.FaskesID != faskesID {
		return nil, ErrPatientNotFound
	}

	now := time.Now().In(wibLocation)
	today := truncateToDay(now)

	cacheKey := fmt.Sprintf("brief:nakes:%s:%s", patientID, today.Format("2006-01-02"))
	if cached := cacheRead[model.PreVisitBriefResponse](ctx, u.Redis, u.Log, cacheKey); cached != nil {
		return cached, nil
	}

	since := today.AddDate(0, 0, -(briefWindowDays - 1)) // 30 hari termasuk hari ini (WIB)

	// --- Komposisi (semua read-only) ---
	earliest, err := u.Repo.GetEarliestLogDate(u.DB, patientID)
	if err != nil {
		return nil, fmt.Errorf("brief earliest log date: %w", err)
	}
	raw, err := u.Repo.GetWindowAggregates(u.DB, patientID, since)
	if err != nil {
		return nil, fmt.Errorf("brief aggregates: %w", err)
	}
	weight, err := u.Repo.GetWeightWindow(u.DB, patientID, since)
	if err != nil {
		return nil, fmt.Errorf("brief weight: %w", err)
	}
	logDates, err := u.Repo.GetLogDatesSince(u.DB, patientID, since)
	if err != nil {
		return nil, fmt.Errorf("brief log dates: %w", err)
	}
	histRows, err := u.HistoryRepo.GetRecordHistory(u.DB, patientID, briefWindowDays)
	if err != nil {
		return nil, fmt.Errorf("brief record history: %w", err)
	}
	medDays, err := u.MedRepo.GetMedAdherenceDays(u.DB, patientID, since)
	if err != nil {
		return nil, fmt.Errorf("brief med adherence days: %w", err)
	}
	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, wibLocation)
	escRows, err := u.EscalationRepo.FindByPatientSince(u.DB, patientID, monthStart)
	if err != nil {
		return nil, fmt.Errorf("brief escalations: %w", err)
	}

	// Risk terbaru best-effort: belum pernah diskor bukan error; kegagalan lain di-log
	// tanpa menggagalkan brief (dokter tetap butuh sisanya).
	var briefRisk *model.BriefRisk
	if rs, riskErr := u.RiskRepo.FindLatestByPatient(u.DB, patientID); riskErr == nil && rs != nil {
		briefRisk = &model.BriefRisk{
			Score:       rs.Score,
			Status:      rs.Status,
			ScoringMode: rs.ScoringMode,
			ScoredAt:    rs.ScoredAt,
			TopFactors:  parseTopFactorStrings(rs.TopFactors),
		}
	} else if riskErr != nil && !errors.Is(riskErr, gorm.ErrRecordNotFound) {
		u.Log.Warn("brief: loading latest risk failed", zap.String("patient_id", patientID), zap.Error(riskErr))
	}

	_, streak := computeStreak(logDates, now)

	daily := mapRecordHistoryRows(filterAndSortAscending(histRows, since))

	resp := &model.PreVisitBriefResponse{
		Period: model.SummaryPeriod{
			Start: since.Format("2006-01-02"),
			End:   today.Format("2006-01-02"),
		},
		Coverage: model.SummaryCoverage{
			LoggedDays: len(logDates),
			WindowDays: briefWindowDays,
			StreakDays: streak,
		},
		HistoryDays: historySpanDays(earliest, today),
		Trajectory: model.BriefTrajectory{
			Daily:                daily,
			GlucoseSlopePerWeek:  glucoseSlopePerWeek(daily),
			SystolicSlopePerWeek: systolicSlopePerWeek(daily),
		},
		Aggregates:           buildSummaryAggregates(raw, weight),
		Risk:                 briefRisk,
		MedAdherence:         buildBriefMedAdherence(medDays),
		EscalationsThisMonth: buildBriefEscalations(escRows),
		GeneratedAt:          now,
	}

	// --- Narasi + draft anamnesis (Gemini). Gagal -> fallback, jangan cache. ---
	out, genErr := u.Generator.GenerateSummary(ctx, buildBriefPrompt(resp))
	if genErr != nil {
		u.Log.Warn("gemini brief generation failed, serving data only",
			zap.String("patient_id", patientID),
			zap.Error(genErr),
		)
		resp.Narrative = briefFallbackNarrative
		return resp, nil
	}
	resp.Narrative, resp.AnamnesisQuestions = parseBriefLLMOutput(out)

	cacheWrite(ctx, u.Redis, u.Log, cacheKey, resp, summaryCacheTTL)
	return resp, nil
}

// parseTopFactorStrings membaca risk_scores.top_factors sebagai []string (kalimat
// penalti dari ML — lihat NOTE di scoring_usecase.go). Payload tak dikenal -> kosong,
// bukan error: kolom ini pernah berganti bentuk dan brief tidak boleh gagal karenanya.
func parseTopFactorStrings(raw json.RawMessage) []string {
	if len(raw) == 0 {
		return []string{}
	}
	var factors []string
	if err := json.Unmarshal(raw, &factors); err != nil {
		return []string{}
	}
	return factors
}

// filterAndSortAscending membuang hari di luar window lalu membalik urutan repo
// (DESC) menjadi kronologis naik untuk grafik & perhitungan slope.
func filterAndSortAscending(rows []repository.RecordHistoryRaw, since time.Time) []repository.RecordHistoryRaw {
	const layout = "2006-01-02"
	sinceStr := since.Format(layout)
	out := make([]repository.RecordHistoryRaw, 0, len(rows))
	for i := len(rows) - 1; i >= 0; i-- {
		if rows[i].LogDate.Format(layout) >= sinceStr {
			out = append(out, rows[i])
		}
	}
	return out
}

func glucoseSlopePerWeek(daily []model.RecordHistoryItem) *float64 {
	points := make([]trendPoint, 0, len(daily))
	for _, d := range daily {
		if d.BloodSugar != nil {
			points = append(points, trendPoint{date: d.Date, value: *d.BloodSugar})
		}
	}
	return slopePerWeek(points)
}

func systolicSlopePerWeek(daily []model.RecordHistoryItem) *float64 {
	points := make([]trendPoint, 0, len(daily))
	for _, d := range daily {
		if d.Systolic != nil {
			points = append(points, trendPoint{date: d.Date, value: float64(*d.Systolic)})
		}
	}
	return slopePerWeek(points)
}

type trendPoint struct {
	date  string // YYYY-MM-DD
	value float64
}

// slopePerWeek menghitung kemiringan tren nilai per minggu dengan least-squares
// sederhana (x = hari sejak titik pertama). Nil bila titik < 3 atau semua di hari sama.
// ponytail: OLS sederhana, cukup untuk arah tren — bukan analisis statistik formal.
func slopePerWeek(points []trendPoint) *float64 {
	if len(points) < 3 {
		return nil
	}
	const layout = "2006-01-02"
	first, err := time.Parse(layout, points[0].date)
	if err != nil {
		return nil
	}

	var sumX, sumY float64
	xs := make([]float64, len(points))
	for i, p := range points {
		d, err := time.Parse(layout, p.date)
		if err != nil {
			return nil
		}
		xs[i] = d.Sub(first).Hours() / 24
		sumX += xs[i]
		sumY += p.value
	}
	meanX := sumX / float64(len(points))
	meanY := sumY / float64(len(points))

	var num, den float64
	for i, p := range points {
		num += (xs[i] - meanX) * (p.value - meanY)
		den += (xs[i] - meanX) * (xs[i] - meanX)
	}
	if den == 0 {
		return nil
	}
	slope := round1(num / den * 7)
	return &slope
}

// indonesianWeekdays diindeks dengan time.Weekday (Minggu = 0).
var indonesianWeekdays = [...]string{"Minggu", "Senin", "Selasa", "Rabu", "Kamis", "Jumat", "Sabtu"}

// buildBriefMedAdherence merangkum hari minum vs lupa obat. Rate dihitung per hari
// (bukan per log) supaya konsisten dengan taken/missed di bagian yang sama; hari tanpa
// log tidak dihitung lupa (tidak diketahui ≠ lupa).
func buildBriefMedAdherence(days []repository.MedDayRaw) model.BriefMedAdherence {
	out := model.BriefMedAdherence{
		MissedDates:    []string{},
		MissedWeekdays: map[string]int{},
	}
	for _, d := range days {
		if d.Taken {
			out.TakenDays++
			continue
		}
		out.MissedDays++
		out.MissedDates = append(out.MissedDates, d.LogDate.Format("2006-01-02"))
		out.MissedWeekdays[indonesianWeekdays[d.LogDate.Weekday()]]++
	}
	if total := out.TakenDays + out.MissedDays; total > 0 {
		out.AdherenceRatePct = round1(float64(out.TakenDays) / float64(total) * 100)
	}
	return out
}

func buildBriefEscalations(rows []repository.EscalationBriefRow) []model.BriefEscalation {
	out := make([]model.BriefEscalation, 0, len(rows))
	for _, r := range rows {
		out = append(out, model.BriefEscalation{
			Tier:     r.Tier,
			Status:   r.Status,
			Feedback: r.Feedback,
			SentAt:   r.SentAt,
			ActedAt:  r.ActedAt,
		})
	}
	return out
}

// buildBriefPrompt menyusun prompt Gemini: seluruh data brief (JSON) + instruksi
// output JSON ketat {narrative, questions} agar bisa diparse jadi dua field response.
func buildBriefPrompt(resp *model.PreVisitBriefResponse) string {
	data := struct {
		Period       model.SummaryPeriod      `json:"period"`
		Coverage     model.SummaryCoverage    `json:"coverage"`
		Trajectory   model.BriefTrajectory    `json:"trajectory"`
		Aggregates   *model.SummaryAggregates `json:"aggregates"`
		Risk         *model.BriefRisk         `json:"risk"`
		MedAdherence model.BriefMedAdherence  `json:"med_adherence"`
		Escalations  []model.BriefEscalation  `json:"escalations_this_month"`
	}{
		Period:       resp.Period,
		Coverage:     resp.Coverage,
		Trajectory:   resp.Trajectory,
		Aggregates:   resp.Aggregates,
		Risk:         resp.Risk,
		MedAdherence: resp.MedAdherence,
		Escalations:  resp.EscalationsThisMonth,
	}
	jsonData, _ := json.Marshal(data)

	instruction := "Anda asisten klinis untuk dokter program Prolanis BPJS (diabetes tipe 2 & hipertensi). " +
		"DATA di bawah adalah rekap 30 hari terakhir seorang pasien menjelang kontrol bulanan (~5 menit). " +
		"Balas HANYA dengan JSON valid berformat {\"narrative\": \"...\", \"questions\": [\"...\"]} tanpa teks lain di luar JSON. " +
		"narrative = ringkasan klinis 3-5 kalimat Bahasa Indonesia: tren gula darah & tekanan darah (pakai slope dan angka konkret), " +
		"kepatuhan obat (termasuk pola hari lupa bila ada), dan eskalasi bulan ini beserta hasilnya. " +
		"questions = 3-5 pertanyaan anamnesis spesifik Bahasa Indonesia yang perlu dokter tanyakan, diturunkan langsung dari temuan di DATA " +
		"(mis. penyebab lupa obat di hari tertentu, pemicu lonjakan gula, keluhan saat eskalasi). " +
		"HANYA gunakan angka yang ada di DATA — jangan mengarang nilai. Abaikan bagian yang tidak ada datanya."

	return instruction + "\n\nDATA (JSON):\n" + string(jsonData)
}

// parseBriefLLMOutput mem-parse output Gemini sebagai JSON {narrative, questions}.
// Toleran terhadap code-fence/teks di sekitar JSON. Gagal parse -> seluruh teks jadi
// narrative dan questions kosong (degradasi anggun, jangan sampai 500 karena format LLM).
func parseBriefLLMOutput(raw string) (string, []string) {
	s := strings.TrimSpace(raw)
	if i := strings.Index(s, "{"); i >= 0 {
		if j := strings.LastIndex(s, "}"); j > i {
			var out struct {
				Narrative string   `json:"narrative"`
				Questions []string `json:"questions"`
			}
			if err := json.Unmarshal([]byte(s[i:j+1]), &out); err == nil && out.Narrative != "" {
				if out.Questions == nil {
					out.Questions = []string{}
				}
				return out.Narrative, out.Questions
			}
		}
	}
	return s, []string{}
}

// cacheRead/cacheWrite — helper cache Redis generik (best-effort, nil client = disabled).
// Dipakai brief & summary agar logika Redis tidak diduplikasi per tipe response.

func cacheRead[T any](ctx context.Context, r *redis.Client, log *zap.Logger, key string) *T {
	if r == nil {
		return nil
	}
	val, err := r.Get(ctx, key).Result()
	if err == redis.Nil {
		return nil
	}
	if err != nil {
		log.Warn("cache read failed", zap.String("key", key), zap.Error(err))
		return nil
	}
	var out T
	if err := json.Unmarshal([]byte(val), &out); err != nil {
		log.Warn("cache unmarshal failed", zap.String("key", key), zap.Error(err))
		return nil
	}
	return &out
}

func cacheWrite[T any](ctx context.Context, r *redis.Client, log *zap.Logger, key string, val *T, ttl time.Duration) {
	if r == nil {
		return
	}
	payload, err := json.Marshal(val)
	if err != nil {
		log.Warn("cache marshal failed", zap.String("key", key), zap.Error(err))
		return
	}
	if err := r.Set(ctx, key, payload, ttl).Err(); err != nil {
		log.Warn("cache write failed", zap.String("key", key), zap.Error(err))
	}
}
