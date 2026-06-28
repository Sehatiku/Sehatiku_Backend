package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sehatiku-backend/internal/entity"
	"sehatiku-backend/internal/model"
	"sehatiku-backend/internal/repository"
	"strings"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

var ErrNoMetricProvided = errors.New("minimal satu metrik harus diisi")

const (
	recordSource   = "app"
	recordLoggedBy = "patient"
)

type recordHealthLogRepo interface {
	Create(db *gorm.DB, log *entity.HealthLog) error
}

type recordHistoryRepo interface {
	GetRecordHistory(db *gorm.DB, patientID string, limit int) ([]repository.RecordHistoryRaw, error)
}

type RecordUseCase struct {
	DB          *gorm.DB
	LogRepo     recordHealthLogRepo
	HistoryRepo recordHistoryRepo
	Log         *zap.Logger
}

// CreateRecord memvalidasi lalu menyimpan catatan harian pasien sebagai satu atau lebih
// baris di health_logs (satu baris per metrik yang diisi). Berbeda dari POST /health-logs
// yang satu request per metrik, endpoint ini untuk form native dengan semua metrik sekaligus.
func (u *RecordUseCase) CreateRecord(ctx context.Context, patientID string, req *model.CreateRecordRequest) (*model.CreateRecordResponse, error) {
	recordedAt, err := parseMeasuredAt(req.RecordedAt)
	if err != nil {
		return nil, err
	}

	created := make([]string, 0, 5)

	if req.BloodSugar != nil {
		if *req.BloodSugar < 20 || *req.BloodSugar > 600 {
			return nil, fmt.Errorf("%w: blood_sugar (mg/dL) harus antara 20 dan 600", ErrInvalidHealthLog)
		}
		if err := u.LogRepo.Create(u.DB, &entity.HealthLog{
			PatientID:    patientID,
			LoggedBy:     recordLoggedBy,
			MetricType:   "glucose",
			ValueNumeric: req.BloodSugar,
			MeasuredAt:   recordedAt,
			Source:       recordSource,
		}); err != nil {
			return nil, fmt.Errorf("inserting glucose record: %w", err)
		}
		created = append(created, "glucose")
	}

	// systolic dan diastolic harus diisi bersama — satu tanpa yang lain ditolak.
	if (req.Systolic == nil) != (req.Diastolic == nil) {
		return nil, fmt.Errorf("%w: systolic dan diastolic harus diisi bersama", ErrInvalidHealthLog)
	}
	if req.Systolic != nil && req.Diastolic != nil {
		sys, dia := *req.Systolic, *req.Diastolic
		if sys < 40 || sys > 300 {
			return nil, fmt.Errorf("%w: systolic harus antara 40 dan 300", ErrInvalidHealthLog)
		}
		if dia < 20 || dia > 200 {
			return nil, fmt.Errorf("%w: diastolic harus antara 20 dan 200", ErrInvalidHealthLog)
		}
		if sys <= dia {
			return nil, fmt.Errorf("%w: systolic harus lebih besar dari diastolic", ErrInvalidHealthLog)
		}
		bpJSON := fmt.Sprintf(`{"systolic":%d,"diastolic":%d}`, sys, dia)
		if err := u.LogRepo.Create(u.DB, &entity.HealthLog{
			PatientID:  patientID,
			LoggedBy:   recordLoggedBy,
			MetricType: "bp",
			ValueJSONB: &bpJSON,
			MeasuredAt: recordedAt,
			Source:     recordSource,
		}); err != nil {
			return nil, fmt.Errorf("inserting bp record: %w", err)
		}
		created = append(created, "bp")
	}

	if req.Weight != nil {
		if *req.Weight < 1 || *req.Weight > 500 {
			return nil, fmt.Errorf("%w: weight (kg) harus antara 1 dan 500", ErrInvalidHealthLog)
		}
		if err := u.LogRepo.Create(u.DB, &entity.HealthLog{
			PatientID:    patientID,
			LoggedBy:     recordLoggedBy,
			MetricType:   "weight",
			ValueNumeric: req.Weight,
			MeasuredAt:   recordedAt,
			Source:       recordSource,
		}); err != nil {
			return nil, fmt.Errorf("inserting weight record: %w", err)
		}
		created = append(created, "weight")
	}

	if req.MedicineTaken != nil {
		adherence := 0.0
		if *req.MedicineTaken {
			adherence = 100.0
		}
		if err := u.LogRepo.Create(u.DB, &entity.HealthLog{
			PatientID:    patientID,
			LoggedBy:     recordLoggedBy,
			MetricType:   "med_adherence",
			ValueNumeric: &adherence,
			MeasuredAt:   recordedAt,
			Source:       recordSource,
		}); err != nil {
			return nil, fmt.Errorf("inserting med_adherence record: %w", err)
		}
		created = append(created, "med_adherence")
	}

	meals := strings.TrimSpace(req.Meals)
	if meals != "" {
		if len(meals) > foodTextMaxLen {
			return nil, fmt.Errorf("%w: meals maksimal %d karakter", ErrInvalidHealthLog, foodTextMaxLen)
		}
		if err := u.LogRepo.Create(u.DB, &entity.HealthLog{
			PatientID:  patientID,
			LoggedBy:   recordLoggedBy,
			MetricType: "food",
			ValueText:  &meals,
			MeasuredAt: recordedAt,
			Source:     recordSource,
		}); err != nil {
			return nil, fmt.Errorf("inserting food record: %w", err)
		}
		created = append(created, "food")
	}

	if len(created) == 0 {
		return nil, ErrNoMetricProvided
	}

	u.Log.Info("daily record created",
		zap.String("patient_id", patientID),
		zap.Int("metric_count", len(created)),
	)

	return &model.CreateRecordResponse{
		RecordedAt: recordedAt,
		Created:    created,
	}, nil
}

// GetHistory mengambil riwayat catatan harian pasien (glucose, bp, weight) untuk grafik.
// Limit dibatasi antara 1 dan 90; default 7 jika di luar rentang.
func (u *RecordUseCase) GetHistory(ctx context.Context, patientID string, limit int) ([]model.RecordHistoryItem, error) {
	if limit <= 0 || limit > 90 {
		limit = 7
	}

	rows, err := u.HistoryRepo.GetRecordHistory(u.DB, patientID, limit)
	if err != nil {
		return nil, fmt.Errorf("getting record history for patient %s: %w", patientID, err)
	}

	items := make([]model.RecordHistoryItem, 0, len(rows))
	for _, row := range rows {
		item := model.RecordHistoryItem{
			Date:       row.LogDate.Format("2006-01-02"),
			BloodSugar: row.BloodSugar,
			Weight:     row.Weight,
		}
		if row.BpRaw != nil {
			var bp struct {
				Systolic  int `json:"systolic"`
				Diastolic int `json:"diastolic"`
			}
			if err := json.Unmarshal([]byte(*row.BpRaw), &bp); err == nil {
				item.Systolic = &bp.Systolic
				item.Diastolic = &bp.Diastolic
			}
		}
		items = append(items, item)
	}
	return items, nil
}
