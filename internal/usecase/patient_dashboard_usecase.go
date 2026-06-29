package usecase

import (
	"context"
	"encoding/json"
	"fmt"
	"sehatiku-backend/internal/entity"
	"sehatiku-backend/internal/model"
	"sehatiku-backend/internal/repository"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

// featureAdvice memetakan feature SHAP (risk_scores.top_factors) ke kalimat anjuran
// Indonesia untuk pasien. Sejalan dengan featureLabels di dashboard_usecase.go.
var featureAdvice = map[string]string{
	"hba1c":              "Kontrol HbA1c Anda dengan rutin minum obat dan menjaga pola makan.",
	"total_sodium_mg":    "Kurangi konsumsi garam dan makanan asin untuk menjaga tekanan darah.",
	"sleep_hours":        "Usahakan tidur cukup 7-8 jam setiap malam.",
	"bmi":                "Jaga berat badan ideal dengan pola makan seimbang dan olahraga.",
	"glucose_avg":        "Pantau gula darah lebih rutin dan catat setiap hari.",
	"glucose_roll7_mean": "Rerata gula darah Anda tinggi minggu ini, jaga asupan karbohidrat.",
	"glucose_max":        "Hindari lonjakan gula darah dengan membatasi makanan manis.",
	"systolic_avg":       "Tekanan darah Anda tinggi, kurangi garam dan kelola stres.",
	"diastolic_avg":      "Jaga tekanan darah dengan istirahat cukup dan batasi garam.",
	"total_sugar_g":      "Kurangi asupan gula pada makanan dan minuman.",
	"activity_minutes":   "Tingkatkan aktivitas fisik, minimal jalan kaki 30 menit sehari.",
	"stress_level":       "Kelola stres dengan istirahat dan aktivitas yang menenangkan.",
	"med_adherence_rate": "Minum obat secara teratur sesuai anjuran dokter.",
	"smoking_flag":       "Berhenti merokok untuk menurunkan risiko komplikasi.",
	"alcohol_flag":       "Hindari konsumsi alkohol untuk menjaga kesehatan Anda.",
}

const (
	onboardingRecommendation = "Mulai catat kondisi kesehatan Anda setiap hari agar kami dapat memantau risiko Anda."
	genericRecommendation    = "Pertahankan pola hidup sehat dan tetap rutin mencatat kondisi Anda."
)

// logDateWindowDays membatasi pengambilan tanggal log untuk perhitungan streak.
const logDateWindowDays = 60

// wibLocation adalah zona waktu Asia/Jakarta (WIB), dipakai untuk menentukan "hari ini"
// dari sudut pandang pasien Indonesia, bukan timezone server. Fallback ke UTC+7 statis
// kalau database timezone tidak tersedia di sistem.
var wibLocation = func() *time.Location {
	loc, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		return time.FixedZone("WIB", 7*60*60)
	}
	return loc
}()

type patientDashboardRepo interface {
	GetLatestRisk(db *gorm.DB, patientID string) (*repository.PatientRiskRow, error)
	GetLatestGlucose(db *gorm.DB, patientID string) (*repository.GlucoseRow, error)
	GetLatestBP(db *gorm.DB, patientID string) (*repository.BPRow, error)
	GetLogDatesSince(db *gorm.DB, patientID string, since time.Time) ([]time.Time, error)
	GetNakesFullName(db *gorm.DB, nakesID string) (string, error)
}

type patientProfileRepo interface {
	FindByCondition(db *gorm.DB, condition string, args ...any) (*entity.Patient, error)
}

type PatientDashboardUseCase struct {
	DB          *gorm.DB
	Repo        patientDashboardRepo
	PatientRepo patientProfileRepo
	Log         *zap.Logger
}

func (u *PatientDashboardUseCase) GetDashboard(ctx context.Context, patientID string) (*model.PatientDashboardResponse, error) {
	patient, err := u.PatientRepo.FindByCondition(u.DB, "id = ?", patientID)
	if err != nil {
		return nil, fmt.Errorf("loading patient profile %s: %w", patientID, err)
	}

	// Ambil nama nakes penanggung jawab; non-fatal bila tidak ditemukan (data integrity issue,
	// bukan error request) — field dibiarkan kosong agar dashboard tetap tampil.
	var assignedNakesName string
	if patient.AssignedNakesID != "" {
		name, err := u.Repo.GetNakesFullName(u.DB, patient.AssignedNakesID)
		if err != nil {
			u.Log.Warn("failed to load assigned nakes name",
				zap.String("patient_id", patientID),
				zap.String("assigned_nakes_id", patient.AssignedNakesID),
				zap.Error(err),
			)
		} else {
			assignedNakesName = name
		}
	}

	riskRow, err := u.Repo.GetLatestRisk(u.DB, patientID)
	if err != nil {
		return nil, fmt.Errorf("patient dashboard risk: %w", err)
	}

	glucoseRow, err := u.Repo.GetLatestGlucose(u.DB, patientID)
	if err != nil {
		return nil, fmt.Errorf("patient dashboard glucose: %w", err)
	}

	bpRow, err := u.Repo.GetLatestBP(u.DB, patientID)
	if err != nil {
		return nil, fmt.Errorf("patient dashboard blood pressure: %w", err)
	}

	since := time.Now().AddDate(0, 0, -logDateWindowDays)
	logDates, err := u.Repo.GetLogDatesSince(u.DB, patientID, since)
	if err != nil {
		return nil, fmt.Errorf("patient dashboard log dates: %w", err)
	}

	loggedToday, streak := computeStreak(logDates, time.Now().In(wibLocation))

	resp := &model.PatientDashboardResponse{
		Profile: model.PatientDashboardProfile{
			FullName:          patient.FullName,
			Age:               calcAge(patient.DateOfBirth),
			DiseaseType:       patient.DiseaseType,
			CompanionName:     patient.CompanionName,
			CompanionPhone:    patient.CompanionPhone,
			AssignedNakesName: assignedNakesName,
		},
		Risk:               buildRiskSection(riskRow),
		LatestMeasurements: buildMeasurements(glucoseRow, bpRow),
		Logging: model.PatientDashboardLogging{
			LoggedToday: loggedToday,
			StreakDays:  streak,
		},
		Recommendations: buildRecommendations(riskRow),
	}
	return resp, nil
}

func buildRiskSection(row *repository.PatientRiskRow) model.PatientDashboardRisk {
	if row == nil {
		return model.PatientDashboardRisk{
			Score:      0,
			RiskLabel:  riskLabel(0),
			Status:     "aman",
			MainFactor: "",
			ScoredAt:   nil,
		}
	}
	scoredAt := row.ScoredAt
	return model.PatientDashboardRisk{
		Score:      row.Score,
		RiskLabel:  riskLabel(row.Score),
		Status:     row.Status,
		MainFactor: extractMainFactor(row.TopFactors),
		ScoredAt:   &scoredAt,
	}
}

func buildMeasurements(glucose *repository.GlucoseRow, bp *repository.BPRow) model.PatientDashboardMeasurements {
	out := model.PatientDashboardMeasurements{}
	if glucose != nil {
		out.Glucose = &model.GlucoseReading{
			Value:      glucose.Value,
			MeasuredAt: glucose.MeasuredAt,
		}
	}
	if bp != nil {
		var parsed struct {
			Systolic  int `json:"systolic"`
			Diastolic int `json:"diastolic"`
		}
		if err := json.Unmarshal(bp.ValueJSONB, &parsed); err == nil {
			out.BloodPressure = &model.BPReading{
				Systolic:   parsed.Systolic,
				Diastolic:  parsed.Diastolic,
				MeasuredAt: bp.MeasuredAt,
			}
		}
	}
	return out
}

func buildRecommendations(row *repository.PatientRiskRow) []string {
	if row == nil || len(row.TopFactors) == 0 {
		return []string{onboardingRecommendation}
	}

	var factors []model.RiskFactor
	if err := json.Unmarshal(row.TopFactors, &factors); err != nil || len(factors) == 0 {
		return []string{genericRecommendation}
	}

	recs := make([]string, 0, 3)
	for _, f := range factors {
		if len(recs) >= 3 {
			break
		}
		if advice, ok := featureAdvice[f.Feature]; ok {
			recs = append(recs, advice)
		}
	}
	if len(recs) == 0 {
		return []string{genericRecommendation}
	}
	return recs
}

// computeStreak menghitung jumlah hari berturut-turut yang punya log, mundur tanpa gap
// dari hari terakhir yang punya log (hari ini jika ada, kalau tidak kemarin). Bernilai 0
// jika log terakhir lebih lama dari kemarin.
func computeStreak(dates []time.Time, now time.Time) (loggedToday bool, streak int) {
	const layout = "2006-01-02"

	set := make(map[string]bool, len(dates))
	for _, d := range dates {
		set[d.Format(layout)] = true
	}

	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
	loggedToday = set[today.Format(layout)]

	start := today
	if !loggedToday {
		start = today.AddDate(0, 0, -1)
		if !set[start.Format(layout)] {
			return loggedToday, 0
		}
	}

	for d := start; set[d.Format(layout)]; d = d.AddDate(0, 0, -1) {
		streak++
	}
	return loggedToday, streak
}
