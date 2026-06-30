package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sehatiku-backend/internal/entity"
	"sehatiku-backend/internal/gateway/ml"
	"sehatiku-backend/internal/model"
	"sehatiku-backend/internal/repository"
	"strings"
	"time"

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
	GetLastLogAt(db *gorm.DB, patientID string) (*time.Time, error)
}

// recordScorer menghitung health score harian (roll-7 + ML). Di-interface-kan agar
// RecordUseCase bisa diuji dengan mock; dipenuhi oleh *ScoringUseCase.
type recordScorer interface {
	ScorePatient(ctx context.Context, patientID string) (*ml.PredictResult, error)
}

type RecordUseCase struct {
	DB          *gorm.DB
	LogRepo     recordHealthLogRepo
	HistoryRepo recordHistoryRepo
	Extractor   foodExtractor // enrich gizi 'meals' lewat NER; boleh nil
	Scorer      recordScorer  // skor harian setelah catatan tersimpan; boleh nil
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
		foodLog := &entity.HealthLog{
			PatientID:  patientID,
			LoggedBy:   recordLoggedBy,
			MetricType: "food",
			ValueText:  &meals,
			MeasuredAt: recordedAt,
			Source:     recordSource,
		}
		// Enrich gizi (NER+TKPI) -> value_jsonb supaya makanan ikut dihitung di roll-7.
		enrichFoodJSONB(ctx, u.Extractor, foodLog, meals, u.Log)
		if err := u.LogRepo.Create(u.DB, foodLog); err != nil {
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

	resp := &model.CreateRecordResponse{
		RecordedAt: recordedAt,
		Created:    created,
	}

	// Skor harian (best-effort): roll-7 + ML setelah catatan tersimpan. Bila gagal
	// (belum ada baseline klinis / ML tak terjangkau), catatan tetap tersimpan dan
	// `score` di-omit dari response (frontend menampilkan "skor belum tersedia").
	if u.Scorer != nil {
		if res, scoreErr := u.Scorer.ScorePatient(ctx, patientID); scoreErr != nil {
			u.Log.Warn("scoring setelah catatan harian gagal (catatan tetap tersimpan)",
				zap.String("patient_id", patientID), zap.Error(scoreErr))
		} else {
			resp.Score = &model.HealthScoreResponse{
				HealthScore:  res.HealthScore,
				Status:       res.Status,
				StatusLabel:  res.StatusLabel,
				Message:      res.Message,
				TopPenalties: res.TopPenalties,
				ScoredAt:     time.Now(),
			}
		}
	}

	return resp, nil
}

// GetTodayStatus mengecek status input harian pasien dari sudut pandang WIB:
//   - logged_today: true jika log terakhir jatuh di tanggal hari ini (WIB).
//   - days_since_last_log: jumlah hari kalender (WIB) sejak input terakhir (0 jika hari ini,
//     1 jika kemarin, dst). nil jika pasien belum pernah mengisi sama sekali.
//   - last_logged_at: waktu input terakhir, nil jika belum pernah.
//
// Dipakai mobile untuk memunculkan pop-up pengingat dan menampilkan "sudah X hari tidak mengisi".
func (u *RecordUseCase) GetTodayStatus(ctx context.Context, patientID string) (*model.TodayStatusResponse, error) {
	lastAt, err := u.HistoryRepo.GetLastLogAt(u.DB, patientID)
	if err != nil {
		return nil, fmt.Errorf("getting today status for patient %s: %w", patientID, err)
	}

	now := time.Now().In(wibLocation)
	resp := &model.TodayStatusResponse{
		Date: now.Format("2006-01-02"),
	}
	if lastAt != nil {
		days := daysBetween(lastAt.In(wibLocation), now)
		resp.LoggedToday = days == 0
		resp.DaysSinceLastLog = &days
		resp.LastLoggedAt = lastAt
	}
	return resp, nil
}

// GetLoggedToday adalah endpoint ringan yang hanya mengembalikan satu boolean:
// true jika pasien sudah mempunyai minimal satu health_log hari ini (WIB), false jika belum.
// Berbeda dari GetTodayStatus yang mengembalikan konteks tambahan (days_since_last_log, dll),
// endpoint ini dipakai ketika mobile hanya butuh jawaban ya/tidak secara efisien.
func (u *RecordUseCase) GetLoggedToday(ctx context.Context, patientID string) (bool, error) {
	lastAt, err := u.HistoryRepo.GetLastLogAt(u.DB, patientID)
	if err != nil {
		return false, fmt.Errorf("getting logged-today for patient %s: %w", patientID, err)
	}
	if lastAt == nil {
		return false, nil
	}
	now := time.Now().In(wibLocation)
	return daysBetween(lastAt.In(wibLocation), now) == 0, nil
}

// daysBetween menghitung selisih hari kalender antara `earlier` dan `later`.
// Keduanya harus sudah berada di zona waktu yang sama (mis. WIB). WIB tidak memiliki DST
// sehingga selisih selalu kelipatan 24 jam tepat.
func daysBetween(earlier, later time.Time) int {
	y1, m1, d1 := earlier.Date()
	y2, m2, d2 := later.Date()
	startEarlier := time.Date(y1, m1, d1, 0, 0, 0, 0, later.Location())
	startLater := time.Date(y2, m2, d2, 0, 0, 0, 0, later.Location())
	return int(startLater.Sub(startEarlier).Hours()) / 24
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
			Date:        row.LogDate.Format("2006-01-02"),
			BloodSugar:  row.BloodSugar,
			Weight:      row.Weight,
			HealthScore: row.HealthScore,
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
