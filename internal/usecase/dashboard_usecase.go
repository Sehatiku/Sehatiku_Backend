package usecase

import (
	"context"
	"encoding/json"
	"fmt"
	"math"
	"sehatiku-backend/internal/model"
	"sehatiku-backend/internal/repository"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

var featureLabels = map[string]string{
	"hba1c":              "HbA1c Tinggi",
	"total_sodium_mg":    "Asupan Natrium Tinggi",
	"sleep_hours":        "Kurang Tidur",
	"bmi":                "BMI Tinggi",
	"glucose_avg":        "Gula Darah Tidak Stabil",
	"glucose_roll7_mean": "Rerata Gula Darah 7 Hari Tinggi",
	"glucose_max":        "Gula Darah Puncak Tinggi",
	"systolic_avg":       "Tekanan Darah Tinggi",
	"diastolic_avg":      "Tekanan Darah Diastolik Tinggi",
	"total_sugar_g":      "Asupan Gula Tinggi",
	"activity_minutes":   "Kurang Aktivitas Fisik",
	"stress_level":       "Stres Tinggi",
	"med_adherence_rate": "Kepatuhan Obat Rendah",
	"smoking_flag":       "Merokok",
	"alcohol_flag":       "Konsumsi Alkohol",
}

type dashboardRepo interface {
	GetSummary(db *gorm.DB, faskesID string) (repository.DashboardSummaryRow, error)
	GetPatientQueue(db *gorm.DB, faskesID string, limit, offset int) ([]repository.PatientQueueRow, int64, error)
}

type DashboardUseCase struct {
	DB            *gorm.DB
	DashboardRepo dashboardRepo
	Log           *zap.Logger
}

func (u *DashboardUseCase) GetSummary(ctx context.Context, faskesID string) (*model.DashboardSummaryResponse, error) {
	row, err := u.DashboardRepo.GetSummary(u.DB, faskesID)
	if err != nil {
		return nil, fmt.Errorf("dashboard summary: %w", err)
	}
	return &model.DashboardSummaryResponse{
		TotalPasien:  row.Total,
		RisikoBahaya: row.Bahaya,
		StatusAman:   row.Aman,
	}, nil
}

func (u *DashboardUseCase) GetPatientQueue(ctx context.Context, faskesID string, page, size int) ([]model.PatientQueueItem, model.PageMetadata, error) {
	offset := (page - 1) * size
	rows, total, err := u.DashboardRepo.GetPatientQueue(u.DB, faskesID, size, offset)
	if err != nil {
		return nil, model.PageMetadata{}, fmt.Errorf("patient queue: %w", err)
	}

	items := make([]model.PatientQueueItem, len(rows))
	for i, row := range rows {
		items[i] = model.PatientQueueItem{
			PatientID:   row.ID,
			FullName:    row.FullName,
			Age:         calcAge(row.DateOfBirth),
			DiseaseType: row.DiseaseType,
			RiskScore:   row.Score,
			RiskLabel:   riskLabel(row.Score),
			Status:      row.RiskStatus,
			MainFactor:  extractMainFactor(row.TopFactors),
		}
	}

	totalPage := int64(math.Ceil(float64(total) / float64(size)))
	paging := model.PageMetadata{
		Page:      page,
		Size:      size,
		TotalItem: total,
		TotalPage: totalPage,
	}
	return items, paging, nil
}

func calcAge(dob *time.Time) int {
	if dob == nil {
		return 0
	}
	now := time.Now()
	age := now.Year() - dob.Year()
	if now.YearDay() < dob.YearDay() {
		age--
	}
	return age
}

func riskLabel(score int) string {
	switch {
	case score >= 80:
		return "kritis"
	case score >= 50:
		return "sedang"
	default:
		return "rendah"
	}
}

func extractMainFactor(raw []byte) string {
	if len(raw) == 0 {
		return ""
	}
	var factors []model.RiskFactor
	if err := json.Unmarshal(raw, &factors); err != nil || len(factors) == 0 {
		return ""
	}
	top := factors[0].Feature
	if label, ok := featureLabels[top]; ok {
		return label
	}
	return top
}
