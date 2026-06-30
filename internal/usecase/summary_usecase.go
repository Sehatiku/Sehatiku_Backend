package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"time"

	"sehatiku-backend/internal/entity"
	"sehatiku-backend/internal/model"
	"sehatiku-backend/internal/repository"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// ErrInvalidWindow dikembalikan saat query param window bukan salah satu dari 7/14/30.
var ErrInvalidWindow = errors.New("window tidak valid, gunakan 7, 14, atau 30")

// allowedWindows adalah window yang didukung endpoint summary (hari).
var allowedWindows = []int{7, 14, 30}

const (
	summaryAudiencePatient = "patient"
	summaryAudienceNakes   = "nakes"

	// summaryCacheTTL — key sudah memuat tanggal WIB, jadi TTL cukup ~1 hari.
	summaryCacheTTL = 24 * time.Hour

	summaryFallbackNarrative = "Ringkasan otomatis sedang tidak tersedia. Silakan lihat angka di bawah untuk memantau kondisi Anda."
)

type summaryRepo interface {
	GetWindowAggregates(db *gorm.DB, patientID string, since time.Time) (*repository.WindowAggregatesRaw, error)
	GetWeightWindow(db *gorm.DB, patientID string, since time.Time) (*repository.WeightWindowRaw, error)
	GetEarliestLogDate(db *gorm.DB, patientID string) (*time.Time, error)
	GetLogDatesSince(db *gorm.DB, patientID string, since time.Time) ([]time.Time, error)
	GetLatestRisk(db *gorm.DB, patientID string) (*repository.SummaryRiskRow, error)
}

type summaryPatientRepo interface {
	FindByID(db *gorm.DB, id string) (*entity.Patient, error)
}

// summaryGenerator adalah dependensi narasi (Gemini). Interface agar bisa di-mock di test.
type summaryGenerator interface {
	GenerateSummary(ctx context.Context, prompt string) (string, error)
}

type SummaryUseCase struct {
	DB          *gorm.DB
	Repo        summaryRepo
	PatientRepo summaryPatientRepo
	Generator   summaryGenerator
	Redis       *redis.Client // boleh nil → cache dinonaktifkan
	Log         *zap.Logger
}

// GetPatientSummary menyusun ringkasan untuk pasien yang sedang login (data sendiri).
func (u *SummaryUseCase) GetPatientSummary(ctx context.Context, patientID string, window int) (*model.SummaryResponse, error) {
	return u.buildSummary(ctx, summaryAudiencePatient, patientID, window)
}

// GetNakesPatientSummary menyusun ringkasan klinis satu pasien untuk nakes. Tenancy:
// pasien milik faskes lain dikembalikan sebagai not-found (tidak bocor lintas tenant),
// pola sama dengan PatientUseCase.GetPatientDetail.
func (u *SummaryUseCase) GetNakesPatientSummary(ctx context.Context, faskesID, patientID string, window int) (*model.SummaryResponse, error) {
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
	return u.buildSummary(ctx, summaryAudienceNakes, patientID, window)
}

func (u *SummaryUseCase) buildSummary(ctx context.Context, audience, patientID string, window int) (*model.SummaryResponse, error) {
	if !isAllowedWindow(window) {
		return nil, ErrInvalidWindow
	}

	now := time.Now().In(wibLocation)
	today := truncateToDay(now)

	// --- Availability gate: window hanya valid jika riwayat data menutupinya ---
	earliest, err := u.Repo.GetEarliestLogDate(u.DB, patientID)
	if err != nil {
		return nil, fmt.Errorf("summary earliest log date: %w", err)
	}
	historyDays := historySpanDays(earliest, today)
	available := windowsForSpan(historyDays)
	if !containsInt(available, window) {
		return &model.SummaryResponse{
			Window:           window,
			Available:        false,
			AvailableWindows: available,
			HistoryDays:      historyDays,
			Message:          insufficientDataMessage(audience, window, historyDays),
			Narrative:        "",
			GeneratedAt:      now,
		}, nil
	}

	// --- Cache lookup (best-effort) ---
	cacheKey := fmt.Sprintf("summary:%s:%s:%d:%s", audience, patientID, window, today.Format("2006-01-02"))
	if cached := u.readCache(ctx, cacheKey); cached != nil {
		return cached, nil
	}

	// --- Agregasi window ---
	since := today.AddDate(0, 0, -(window - 1)) // window hari, termasuk hari ini (WIB)

	raw, err := u.Repo.GetWindowAggregates(u.DB, patientID, since)
	if err != nil {
		return nil, fmt.Errorf("summary aggregates: %w", err)
	}
	weight, err := u.Repo.GetWeightWindow(u.DB, patientID, since)
	if err != nil {
		return nil, fmt.Errorf("summary weight: %w", err)
	}
	logDates, err := u.Repo.GetLogDatesSince(u.DB, patientID, since)
	if err != nil {
		return nil, fmt.Errorf("summary log dates: %w", err)
	}
	riskRow, err := u.Repo.GetLatestRisk(u.DB, patientID)
	if err != nil {
		return nil, fmt.Errorf("summary risk: %w", err)
	}

	_, streak := computeStreak(logDates, now)

	resp := &model.SummaryResponse{
		Window:           window,
		Available:        true,
		AvailableWindows: available,
		HistoryDays:      historyDays,
		Period: &model.SummaryPeriod{
			Start: since.Format("2006-01-02"),
			End:   today.Format("2006-01-02"),
		},
		Coverage: &model.SummaryCoverage{
			LoggedDays: len(logDates),
			WindowDays: window,
			StreakDays: streak,
		},
		Aggregates:  buildSummaryAggregates(raw, weight),
		Risk:        buildSummaryRisk(riskRow),
		GeneratedAt: now,
	}

	// --- Narasi (Gemini). Degradasi anggun: gagal -> fallback, jangan cache. ---
	narrative, genErr := u.Generator.GenerateSummary(ctx, buildSummaryPrompt(audience, window, resp))
	if genErr != nil {
		u.Log.Warn("gemini summary generation failed, serving aggregates only",
			zap.String("patient_id", patientID),
			zap.String("audience", audience),
			zap.Int("window", window),
			zap.Error(genErr),
		)
		resp.Narrative = summaryFallbackNarrative
		return resp, nil
	}
	resp.Narrative = narrative

	u.writeCache(ctx, cacheKey, resp)
	return resp, nil
}

// readCache mengembalikan response dari Redis bila ada; nil bila miss/error/disabled.
func (u *SummaryUseCase) readCache(ctx context.Context, key string) *model.SummaryResponse {
	if u.Redis == nil {
		return nil
	}
	val, err := u.Redis.Get(ctx, key).Result()
	if err == redis.Nil {
		return nil
	}
	if err != nil {
		u.Log.Warn("summary cache read failed", zap.String("key", key), zap.Error(err))
		return nil
	}
	var resp model.SummaryResponse
	if err := json.Unmarshal([]byte(val), &resp); err != nil {
		u.Log.Warn("summary cache unmarshal failed", zap.String("key", key), zap.Error(err))
		return nil
	}
	return &resp
}

func (u *SummaryUseCase) writeCache(ctx context.Context, key string, resp *model.SummaryResponse) {
	if u.Redis == nil {
		return
	}
	payload, err := json.Marshal(resp)
	if err != nil {
		u.Log.Warn("summary cache marshal failed", zap.String("key", key), zap.Error(err))
		return
	}
	if err := u.Redis.Set(ctx, key, payload, summaryCacheTTL).Err(); err != nil {
		u.Log.Warn("summary cache write failed", zap.String("key", key), zap.Error(err))
	}
}

func isAllowedWindow(w int) bool { return containsInt(allowedWindows, w) }

func containsInt(xs []int, v int) bool {
	for _, x := range xs {
		if x == v {
			return true
		}
	}
	return false
}

func truncateToDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}

// historySpanDays menghitung rentang hari riwayat pencatatan pasien: dari log pertama
// (WIB) s.d. hari ini, inklusif. 0 jika pasien belum pernah mengisi.
func historySpanDays(earliest *time.Time, today time.Time) int {
	if earliest == nil {
		return 0
	}
	earliestDay := truncateToDay(earliest.In(wibLocation))
	return int(today.Sub(earliestDay).Hours()/24) + 1
}

// windowsForSpan mengembalikan window (7/14/30) yang ditopang rentang riwayat:
// sebuah window w valid jika spanDays >= w.
func windowsForSpan(spanDays int) []int {
	out := make([]int, 0, len(allowedWindows))
	for _, w := range allowedWindows {
		if spanDays >= w {
			out = append(out, w)
		}
	}
	return out
}

// insufficientDataMessage menyusun pesan ramah saat window yang diminta belum ditopang data.
func insufficientDataMessage(audience string, window, historyDays int) string {
	subject := "Data Anda"
	if audience == summaryAudienceNakes {
		subject = "Data pasien"
	}
	if historyDays <= 0 {
		if audience == summaryAudienceNakes {
			return fmt.Sprintf("Pasien belum memiliki data pencatatan. Ringkasan %d hari tersedia setelah ada minimal %d hari pencatatan.", window, window)
		}
		return fmt.Sprintf("Belum ada data kesehatan yang tercatat. Mulai catat kondisi Anda setiap hari — ringkasan %d hari tersedia setelah Anda mencatat minimal %d hari.", window, window)
	}
	return fmt.Sprintf("%s baru mencakup %d hari, sedangkan ringkasan %d hari membutuhkan minimal %d hari pencatatan. Terus catat kondisi harian agar ringkasan ini tersedia.", subject, historyDays, window, window)
}

func buildSummaryAggregates(raw *repository.WindowAggregatesRaw, weight *repository.WeightWindowRaw) *model.SummaryAggregates {
	agg := &model.SummaryAggregates{}

	if raw.GlucoseCount > 0 {
		agg.Glucose = &model.GlucoseAggregate{
			AvgMgDl: round1(deref(raw.GlucoseAvg)),
			MinMgDl: round1(deref(raw.GlucoseMin)),
			MaxMgDl: round1(deref(raw.GlucoseMax)),
			Count:   raw.GlucoseCount,
		}
	}
	if raw.BPCount > 0 {
		agg.BloodPressure = &model.BPAggregate{
			AvgSystolic:  round1(deref(raw.BPSysAvg)),
			AvgDiastolic: round1(deref(raw.BPDiaAvg)),
			Count:        raw.BPCount,
		}
	}
	if raw.MedCount > 0 {
		agg.MedAdherence = &model.MedAdherenceAggregate{
			AdherenceRatePct: round1(deref(raw.MedRate)),
			Count:            raw.MedCount,
		}
	}
	if raw.FoodCount > 0 {
		days := raw.FoodDays
		if days < 1 {
			days = 1
		}
		agg.Nutrition = &model.NutritionAggregate{
			AvgKcalPerDay:     round1(deref(raw.FoodKcalSum) / float64(days)),
			AvgCarbsGPerDay:   round1(deref(raw.FoodCarbsSum) / float64(days)),
			AvgSodiumMgPerDay: round1(deref(raw.FoodSodiumSum) / float64(days)),
			MealCount:         raw.FoodCount,
		}
	}
	if raw.ActivityCount > 0 {
		days := raw.ActivityDays
		if days < 1 {
			days = 1
		}
		agg.Activity = &model.ActivityAggregate{
			AvgMinutesPerDay: round1(deref(raw.ActivitySum) / float64(days)),
			TotalMinutes:     round1(deref(raw.ActivitySum)),
			Count:            raw.ActivityCount,
		}
	}
	if raw.SleepCount > 0 {
		agg.Sleep = &model.SleepAggregate{
			AvgHours: round1(deref(raw.SleepAvg)),
			Count:    raw.SleepCount,
		}
	}
	if raw.StressCount > 0 {
		agg.Stress = &model.StressAggregate{
			AvgLevel: round1(deref(raw.StressAvg)),
			Count:    raw.StressCount,
		}
	}
	if weight.WeightCount > 0 {
		start := deref(weight.StartKg)
		latest := deref(weight.LatestKg)
		agg.Weight = &model.WeightAggregate{
			StartKg:  round1(start),
			LatestKg: round1(latest),
			DeltaKg:  round1(latest - start),
			Count:    weight.WeightCount,
		}
	}
	return agg
}

func buildSummaryRisk(row *repository.SummaryRiskRow) *model.SummaryRisk {
	if row == nil {
		return nil
	}
	score := row.Score
	scoredAt := row.ScoredAt
	return &model.SummaryRisk{
		Score:    &score,
		Status:   row.Status,
		ScoredAt: &scoredAt,
	}
}

// buildSummaryPrompt menyusun prompt audiens-spesifik berisi angka agregat (JSON).
// Instruksi menegaskan agar Gemini hanya memakai angka yang diberikan.
func buildSummaryPrompt(audience string, window int, resp *model.SummaryResponse) string {
	data := struct {
		WindowDays int                      `json:"window_days"`
		Period     *model.SummaryPeriod     `json:"period"`
		Coverage   *model.SummaryCoverage   `json:"coverage"`
		Aggregates *model.SummaryAggregates `json:"aggregates"`
		Risk       *model.SummaryRisk       `json:"risk"`
	}{
		WindowDays: window,
		Period:     resp.Period,
		Coverage:   resp.Coverage,
		Aggregates: resp.Aggregates,
		Risk:       resp.Risk,
	}
	jsonData, _ := json.Marshal(data)

	var instruction string
	if audience == summaryAudienceNakes {
		instruction = fmt.Sprintf(
			"Anda asisten klinis untuk tenaga kesehatan (dokter/kader) program Prolanis BPJS "+
				"(diabetes tipe 2 & hipertensi). Berdasarkan DATA ringkasan %d hari seorang pasien di bawah, "+
				"tulis ringkasan klinis ringkas 3-5 kalimat dalam Bahasa Indonesia. Soroti tren dan flag risiko "+
				"(mis. rerata tekanan darah/gula tinggi, kepatuhan obat rendah, variabilitas gula, penurunan/kenaikan berat). "+
				"Sebutkan angka konkret. HANYA gunakan angka yang ada di DATA — jangan mengarang nilai. "+
				"Abaikan metrik yang tidak ada datanya. Jangan menambahkan pembuka/penutup basa-basi.",
			window,
		)
	} else {
		instruction = fmt.Sprintf(
			"Anda asisten kesehatan ramah untuk pasien program Prolanis (diabetes/hipertensi). "+
				"Berdasarkan DATA ringkasan %d hari di bawah, tulis ringkasan singkat 3-5 kalimat dalam Bahasa "+
				"Indonesia yang mudah dipahami orang awam, memotivasi, dengan sapaan 'Anda'. Soroti hal penting "+
				"(gula darah, tekanan darah, kepatuhan minum obat, pola makan, aktivitas, tidur) dan beri 1 saran "+
				"praktis. Jangan membuat diagnosis medis. HANYA gunakan angka yang ada di DATA — jangan mengarang nilai. "+
				"Abaikan metrik yang tidak ada datanya.",
			window,
		)
	}

	return instruction + "\n\nDATA (JSON):\n" + string(jsonData)
}

func deref(p *float64) float64 {
	if p == nil {
		return 0
	}
	return *p
}

func round1(v float64) float64 {
	return math.Round(v*10) / 10
}
