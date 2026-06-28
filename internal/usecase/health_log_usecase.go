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
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

// ErrInvalidHealthLog dikembalikan bila body input gagal validasi per-metric (nilai wajib
// tidak ada, di luar range, atau measured_at tidak valid). Dipetakan ke HTTP 400.
var ErrInvalidHealthLog = errors.New("data health log tidak valid")

// Re-export error guard repo agar controller cukup bergantung ke package usecase.
var (
	ErrTooManySubmissions  = repository.ErrTooManySubmissions
	ErrIdempotencyInFlight = repository.ErrIdempotencyInFlight
)

const (
	healthLogLoggedBy = "patient" // pasien input sendiri lewat app; pendamping pakai WhatsApp
	healthLogSource   = "web"     // Patient App native (satu-satunya opsi non-WA/SMS di enum log_source)

	// measuredAtSkew memberi toleransi clock skew sebelum menolak timestamp masa depan.
	measuredAtSkew = 5 * time.Minute

	foodTextMaxLen = 500
)

type healthLogRepository interface {
	Create(db *gorm.DB, log *entity.HealthLog) error
	FindByID(db *gorm.DB, id string) (*entity.HealthLog, error)
}

type healthLogGuardRepository interface {
	CheckSubmissionRateLimit(ctx context.Context, patientID string) error
	ReserveIdempotency(ctx context.Context, key string) (existingLogID string, isNew bool, err error)
	CommitIdempotency(ctx context.Context, key, logID string) error
	ReleaseIdempotency(ctx context.Context, key string) error
}

type HealthLogUseCase struct {
	DB            *gorm.DB
	HealthLogRepo healthLogRepository
	GuardRepo     healthLogGuardRepository
	Log           *zap.Logger
}

// CreateHealthLog memvalidasi lalu menyimpan satu pengukuran harian pasien ke health_logs.
// Alur: rate limit -> validasi per-metric -> parse measured_at -> reservasi Idempotency-Key
// -> insert -> commit key. Idempotency mencegah double insert saat double-tap di koneksi flaky.
func (u *HealthLogUseCase) CreateHealthLog(ctx context.Context, patientID, idempotencyKey string, req *model.CreateHealthLogRequest) (*model.HealthLogResponse, error) {
	if err := u.GuardRepo.CheckSubmissionRateLimit(ctx, patientID); err != nil {
		return nil, err
	}

	log := &entity.HealthLog{
		PatientID:  patientID,
		LoggedBy:   healthLogLoggedBy,
		MetricType: req.MetricType,
		Source:     healthLogSource,
	}
	if err := applyMetricValue(req, log); err != nil {
		return nil, err
	}

	measuredAt, err := parseMeasuredAt(req.MeasuredAt)
	if err != nil {
		return nil, err
	}
	log.MeasuredAt = measuredAt

	existingID, isNew, err := u.GuardRepo.ReserveIdempotency(ctx, idempotencyKey)
	if err != nil {
		return nil, fmt.Errorf("checking idempotency: %w", err)
	}
	if !isNew {
		if existingID == "" || existingID == "PENDING" {
			return nil, ErrIdempotencyInFlight
		}
		// Request duplikat (Idempotency-Key sama): ambil baris yang sudah tersimpan supaya
		// response identik dengan yang pertama, tanpa insert ulang.
		existing, err := u.HealthLogRepo.FindByID(u.DB, existingID)
		if err != nil {
			return nil, fmt.Errorf("loading idempotent health log %s: %w", existingID, err)
		}
		return buildHealthLogResponse(existing), nil
	}

	if err := u.HealthLogRepo.Create(u.DB, log); err != nil {
		// Insert gagal: lepas reservasi supaya client bisa retry dengan key yang sama.
		if relErr := u.GuardRepo.ReleaseIdempotency(ctx, idempotencyKey); relErr != nil {
			u.Log.Warn("failed to release idempotency key after insert error",
				zap.String("patient_id", patientID), zap.Error(relErr))
		}
		return nil, fmt.Errorf("inserting health log: %w", err)
	}

	if err := u.GuardRepo.CommitIdempotency(ctx, idempotencyKey, log.ID); err != nil {
		// Insert sudah sukses; commit key gagal hanya melemahkan dedupe (retry bisa insert
		// dobel) — jangan gagalkan request yang datanya sudah tersimpan.
		u.Log.Warn("failed to commit idempotency key after insert",
			zap.String("patient_id", patientID), zap.String("health_log_id", log.ID), zap.Error(err))
	}

	u.Log.Info("health log created",
		zap.String("patient_id", patientID),
		zap.String("health_log_id", log.ID),
		zap.String("metric_type", log.MetricType),
	)

	return buildHealthLogResponse(log), nil
}

// applyMetricValue memvalidasi nilai sesuai metric_type dan mengisinya ke entity (value_numeric
// / value_text / value_jsonb). Range mengikuti tabel validasi di plan & docs/erd.md.
func applyMetricValue(req *model.CreateHealthLogRequest, log *entity.HealthLog) error {
	switch req.MetricType {
	case "glucose":
		return setNumeric(req, log, 20, 600, "glucose (mg/dL) harus antara 20 dan 600")
	case "med_adherence":
		return setNumeric(req, log, 0, 100, "med_adherence (%) harus antara 0 dan 100")
	case "activity":
		return setNumeric(req, log, 0, 1440, "activity (menit) harus antara 0 dan 1440")
	case "sleep":
		return setNumeric(req, log, 0, 24, "sleep (jam) harus antara 0 dan 24")
	case "stress":
		return setNumeric(req, log, 1, 10, "stress harus antara 1 dan 10")
	case "smoking":
		return setNumeric(req, log, 0, 200, "smoking (batang) harus antara 0 dan 200")
	case "alcohol":
		return setNumeric(req, log, 0, 100, "alcohol (unit) harus antara 0 dan 100")
	case "bp":
		return setBloodPressure(req, log)
	case "food":
		return setFood(req, log)
	default:
		return fmt.Errorf("%w: metric_type tidak dikenal", ErrInvalidHealthLog)
	}
}

func setNumeric(req *model.CreateHealthLogRequest, log *entity.HealthLog, min, max float64, msg string) error {
	if req.ValueNumeric == nil {
		return fmt.Errorf("%w: value_numeric wajib diisi untuk metric_type %s", ErrInvalidHealthLog, req.MetricType)
	}
	v := *req.ValueNumeric
	if v < min || v > max {
		return fmt.Errorf("%w: %s", ErrInvalidHealthLog, msg)
	}
	log.ValueNumeric = &v
	return nil
}

func setBloodPressure(req *model.CreateHealthLogRequest, log *entity.HealthLog) error {
	if req.Systolic == nil || req.Diastolic == nil {
		return fmt.Errorf("%w: systolic dan diastolic wajib diisi untuk metric_type bp", ErrInvalidHealthLog)
	}
	sys, dia := *req.Systolic, *req.Diastolic
	if sys < 40 || sys > 300 {
		return fmt.Errorf("%w: systolic harus antara 40 dan 300", ErrInvalidHealthLog)
	}
	if dia < 20 || dia > 200 {
		return fmt.Errorf("%w: diastolic harus antara 20 dan 200", ErrInvalidHealthLog)
	}
	if sys <= dia {
		return fmt.Errorf("%w: systolic harus lebih besar dari diastolic", ErrInvalidHealthLog)
	}
	// Simpan sebagai value_jsonb {"systolic": N, "diastolic": N} — konvensi bp di docs/erd.md,
	// dibaca dashboard pasien dari value_jsonb.
	jsonb := fmt.Sprintf(`{"systolic":%d,"diastolic":%d}`, sys, dia)
	log.ValueJSONB = &jsonb
	return nil
}

func setFood(req *model.CreateHealthLogRequest, log *entity.HealthLog) error {
	text := strings.TrimSpace(req.ValueText)
	if text == "" {
		return fmt.Errorf("%w: value_text wajib diisi untuk metric_type food", ErrInvalidHealthLog)
	}
	if len(text) > foodTextMaxLen {
		return fmt.Errorf("%w: value_text maksimal %d karakter", ErrInvalidHealthLog, foodTextMaxLen)
	}
	// value_jsonb (hasil NER makanan) sengaja dibiarkan null — parsing NER di luar scope endpoint ini.
	log.ValueText = &text
	return nil
}

func parseMeasuredAt(raw string) (time.Time, error) {
	t, err := time.Parse(time.RFC3339, raw)
	if err != nil {
		return time.Time{}, fmt.Errorf("%w: measured_at harus format RFC3339/ISO 8601", ErrInvalidHealthLog)
	}
	if t.After(time.Now().Add(measuredAtSkew)) {
		return time.Time{}, fmt.Errorf("%w: measured_at tidak boleh di masa depan", ErrInvalidHealthLog)
	}
	return t, nil
}

// buildHealthLogResponse memetakan entity (hasil insert atau hasil load duplikat) ke response.
// bp dibaca balik dari value_jsonb agar konsisten antara insert pertama & request duplikat.
func buildHealthLogResponse(log *entity.HealthLog) *model.HealthLogResponse {
	resp := &model.HealthLogResponse{
		ID:         log.ID,
		PatientID:  log.PatientID,
		MetricType: log.MetricType,
		MeasuredAt: log.MeasuredAt,
		LoggedBy:   log.LoggedBy,
		Source:     log.Source,
		CreatedAt:  log.CreatedAt,
	}
	switch log.MetricType {
	case "bp":
		if log.ValueJSONB != nil {
			var bp model.BPValue
			if err := json.Unmarshal([]byte(*log.ValueJSONB), &bp); err == nil {
				resp.BloodPressure = &bp
			}
		}
	case "food":
		if log.ValueText != nil {
			resp.ValueText = *log.ValueText
		}
	default:
		resp.ValueNumeric = log.ValueNumeric
	}
	return resp
}
