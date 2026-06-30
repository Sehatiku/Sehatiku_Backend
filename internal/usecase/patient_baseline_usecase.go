package usecase

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sehatiku-backend/internal/entity"
	"sehatiku-backend/internal/model"
	"sehatiku-backend/internal/repository"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

// ErrBaselineNotFound dikembalikan saat pasien belum punya baseline sama sekali.
var ErrBaselineNotFound = errors.New("baseline pasien belum tersedia")

// ErrInvalidRecordedAt dikembalikan saat field recorded_at bukan format YYYY-MM-DD.
var ErrInvalidRecordedAt = errors.New("format recorded_at tidak valid (gunakan YYYY-MM-DD)")

type PatientBaselineUseCase struct {
	DB            *gorm.DB
	BaselineRepo  baselineHistoryRepo
	PatientRepo   baselinePatientRepo
	NakesRepo     baselineNakesRepo
	RiskScoreRepo baselineRiskScoreRepo
	Log           *zap.Logger
}

type baselineHistoryRepo interface {
	Create(db *gorm.DB, baseline *entity.PatientClinicalBaseline) error
	FindLatestByPatient(db *gorm.DB, patientID string) (*entity.PatientClinicalBaseline, error)
	ListByPatient(db *gorm.DB, patientID string, limit, offset int) ([]entity.PatientClinicalBaseline, int64, error)
}

type baselinePatientRepo interface {
	FindByID(db *gorm.DB, id string) (*entity.Patient, error)
}

type baselineNakesRepo interface {
	FindByID(db *gorm.DB, id string) (*entity.Nakes, error)
	FindByIDs(db *gorm.DB, ids []string) ([]entity.Nakes, error)
}

type baselineRiskScoreRepo interface {
	ListByPatient(db *gorm.DB, patientID string, limit int) ([]repository.RiskScoreHistoryRow, error)
}

// GetLatestBaseline mengembalikan baseline TERLENGKAP terbaru milik pasien (untuk pre-fill
// form update). faskesID dari JWT menjaga tenant isolation: pasien faskes lain -> not-found.
func (u *PatientBaselineUseCase) GetLatestBaseline(ctx context.Context, faskesID, patientID string) (*model.BaselineDetailResponse, error) {
	if _, err := u.ensurePatientOwned(faskesID, patientID); err != nil {
		return nil, err
	}

	baseline, err := u.BaselineRepo.FindLatestByPatient(u.DB, patientID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrBaselineNotFound
		}
		return nil, fmt.Errorf("finding latest baseline: %w", err)
	}

	return u.toBaselineDetail(baseline, u.resolveNakesName(baseline.RecordedByNakesID)), nil
}

// CreateBaseline mencatat versi baseline BARU (insert-only). Tidak menimpa baseline lama;
// baris terbaru menjadi baseline aktif yang dipakai skoring ML berikutnya.
func (u *PatientBaselineUseCase) CreateBaseline(ctx context.Context, faskesID, patientID string, req *model.CreateBaselineRequest) (*model.BaselineDetailResponse, error) {
	if _, err := u.ensurePatientOwned(faskesID, patientID); err != nil {
		return nil, err
	}

	// recorded_by_nakes_id wajib merujuk nakes milik faskes ini (isolasi tenant).
	nakes, err := u.NakesRepo.FindByID(u.DB, req.RecordedByNakesID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrAssignedNakesInvalid
		}
		return nil, fmt.Errorf("checking recorded_by nakes: %w", err)
	}
	if nakes.FaskesID != faskesID {
		return nil, ErrAssignedNakesInvalid
	}

	recordedAt := time.Now()
	if req.RecordedAt != "" {
		parsed, err := time.Parse("2006-01-02", req.RecordedAt)
		if err != nil {
			return nil, ErrInvalidRecordedAt
		}
		recordedAt = parsed
	}

	baseline := buildBaseline(patientID, req.Baseline)
	baseline.RecordedAt = recordedAt
	recordedBy := req.RecordedByNakesID
	baseline.RecordedByNakesID = &recordedBy
	if req.Notes != "" {
		notes := req.Notes
		baseline.Notes = &notes
	}

	if err := u.BaselineRepo.Create(u.DB, baseline); err != nil {
		return nil, fmt.Errorf("creating baseline: %w", err)
	}

	u.Log.Info("baseline recorded",
		zap.String("patient_id", patientID),
		zap.String("baseline_id", baseline.ID),
		zap.String("recorded_by_nakes_id", req.RecordedByNakesID))

	return u.toBaselineDetail(baseline, nakes.FullName), nil
}

// ListBaselineHistoryForFaskes mengembalikan progress baseline (metrik kunci, paginated)
// milik pasien BESERTA tren health score (deret terpisah, dibatasi scoreLimit), terbaru-dulu.
// Tenant isolation lewat faskesID.
func (u *PatientBaselineUseCase) ListBaselineHistoryForFaskes(ctx context.Context, faskesID, patientID string, page, size, scoreLimit int) (*model.BaselineHistoryResponse, model.PageMetadata, error) {
	if _, err := u.ensurePatientOwned(faskesID, patientID); err != nil {
		return nil, model.PageMetadata{}, err
	}

	items, paging, err := u.listHistory(patientID, page, size)
	if err != nil {
		return nil, model.PageMetadata{}, err
	}

	scores, err := u.RiskScoreRepo.ListByPatient(u.DB, patientID, scoreLimit)
	if err != nil {
		return nil, model.PageMetadata{}, fmt.Errorf("listing health score history: %w", err)
	}
	points := make([]model.HealthScorePoint, len(scores))
	for i := range scores {
		points[i] = model.HealthScorePoint{
			Score:    scores[i].Score,
			Status:   scores[i].Status,
			ScoredAt: scores[i].ScoredAt,
		}
	}

	return &model.BaselineHistoryResponse{
		BaselineHistory:    items,
		HealthScoreHistory: points,
	}, paging, nil
}

// ListBaselineHistoryForPatient mengembalikan progress baseline pasien itu sendiri
// (patientID dari JWT pasien), paginated terbaru-dulu.
func (u *PatientBaselineUseCase) ListBaselineHistoryForPatient(ctx context.Context, patientID string, page, size int) ([]model.BaselineHistoryItem, model.PageMetadata, error) {
	return u.listHistory(patientID, page, size)
}

func (u *PatientBaselineUseCase) listHistory(patientID string, page, size int) ([]model.BaselineHistoryItem, model.PageMetadata, error) {
	offset := (page - 1) * size
	baselines, total, err := u.BaselineRepo.ListByPatient(u.DB, patientID, size, offset)
	if err != nil {
		return nil, model.PageMetadata{}, fmt.Errorf("listing baseline history: %w", err)
	}

	names := u.resolveNakesNames(baselines)
	items := make([]model.BaselineHistoryItem, len(baselines))
	for i := range baselines {
		b := &baselines[i]
		var name string
		if b.RecordedByNakesID != nil {
			name = names[*b.RecordedByNakesID]
		}
		items[i] = toBaselineHistoryItem(b, name)
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

// ensurePatientOwned memuat pasien dan memastikan ia milik faskes yang sedang login.
// Pasien tidak ada / milik faskes lain dikembalikan sebagai ErrPatientNotFound (404) agar
// keberadaannya tidak bocor lintas tenant (pola sama dengan PatientUseCase.GetPatientDetail).
func (u *PatientBaselineUseCase) ensurePatientOwned(faskesID, patientID string) (*entity.Patient, error) {
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
	return patient, nil
}

// resolveNakesName mengembalikan nama satu nakes (best-effort; kosong bila nil/tidak ada).
func (u *PatientBaselineUseCase) resolveNakesName(nakesID *string) string {
	if nakesID == nil {
		return ""
	}
	nakes, err := u.NakesRepo.FindByID(u.DB, *nakesID)
	if err != nil {
		u.Log.Warn("recorder nakes not found for baseline", zap.String("nakes_id", *nakesID), zap.Error(err))
		return ""
	}
	return nakes.FullName
}

// resolveNakesNames me-resolve nama recorder secara batch (satu query) untuk seluruh baris.
func (u *PatientBaselineUseCase) resolveNakesNames(baselines []entity.PatientClinicalBaseline) map[string]string {
	idSet := make(map[string]struct{})
	for i := range baselines {
		if baselines[i].RecordedByNakesID != nil {
			idSet[*baselines[i].RecordedByNakesID] = struct{}{}
		}
	}
	if len(idSet) == 0 {
		return nil
	}
	ids := make([]string, 0, len(idSet))
	for id := range idSet {
		ids = append(ids, id)
	}
	list, err := u.NakesRepo.FindByIDs(u.DB, ids)
	if err != nil {
		u.Log.Warn("batch resolving recorder nakes failed", zap.Error(err))
		return nil
	}
	names := make(map[string]string, len(list))
	for i := range list {
		names[list[i].ID] = list[i].FullName
	}
	return names
}

func (u *PatientBaselineUseCase) toBaselineDetail(b *entity.PatientClinicalBaseline, nakesName string) *model.BaselineDetailResponse {
	return &model.BaselineDetailResponse{
		ID:                    b.ID,
		PatientID:             b.PatientID,
		RecordedAt:            b.RecordedAt,
		RecordedByNakesID:     b.RecordedByNakesID,
		RecordedByNakesName:   nakesName,
		Notes:                 b.Notes,
		AgeYears:              b.AgeYears,
		Sex:                   b.Sex,
		BMI:                   b.BMI,
		BMICategory:           b.BMICategory,
		WaistCircumferenceCm:  b.WaistCircumferenceCm,
		CentralObesity:        b.CentralObesity,
		SmokingStatus:         b.SmokingStatus,
		AlcoholUse:            b.AlcoholUse,
		PhysicalActivity:      b.PhysicalActivity,
		FamilyHistoryDiabetes: b.FamilyHistoryDiabetes,
		FamilyHistoryCVD:      b.FamilyHistoryCVD,
		SystolicBPMmhg:        b.SystolicBPMmhg,
		DiastolicBPMmhg:       b.DiastolicBPMmhg,
		HypertensionStatus:    b.HypertensionStatus,
		FastingGlucoseMgdl:    b.FastingGlucoseMgdl,
		HbA1cPct:              b.HbA1cPct,
		DiabetesStatus:        b.DiabetesStatus,
		TotalCholesterolMgdl:  b.TotalCholesterolMgdl,
		HDLMgdl:               b.HDLMgdl,
		LDLMgdl:               b.LDLMgdl,
		TriglyceidesMgdl:      b.TriglyceidesMgdl,
		CVDRisk10YrPct:        b.CVDRisk10YrPct,
		CVDRiskCategory:       b.CVDRiskCategory,
		OnAntihypertensive:    b.OnAntihypertensive,
		OnAntidiabetic:        b.OnAntidiabetic,
		OnStatin:              b.OnStatin,
		TargetRisk:            b.TargetRisk,
		EGFR:                  b.EGFR,
		UACR:                  b.UACR,
		ClusterID:             b.ClusterID,
		DiagnosisCluster:      b.DiagnosisCluster,
		ClinicalGroup:         b.ClinicalGroup,
	}
}

func toBaselineHistoryItem(b *entity.PatientClinicalBaseline, nakesName string) model.BaselineHistoryItem {
	return model.BaselineHistoryItem{
		ID:                   b.ID,
		RecordedAt:           b.RecordedAt,
		RecordedByNakesName:  nakesName,
		Notes:                b.Notes,
		BMI:                  b.BMI,
		BMICategory:          b.BMICategory,
		SystolicBPMmhg:       b.SystolicBPMmhg,
		DiastolicBPMmhg:      b.DiastolicBPMmhg,
		HypertensionStatus:   b.HypertensionStatus,
		FastingGlucoseMgdl:   b.FastingGlucoseMgdl,
		HbA1cPct:             b.HbA1cPct,
		DiabetesStatus:       b.DiabetesStatus,
		TotalCholesterolMgdl: b.TotalCholesterolMgdl,
		HDLMgdl:              b.HDLMgdl,
		LDLMgdl:              b.LDLMgdl,
		TriglyceidesMgdl:     b.TriglyceidesMgdl,
		CVDRisk10YrPct:       b.CVDRisk10YrPct,
		CVDRiskCategory:      b.CVDRiskCategory,
		EGFR:                 b.EGFR,
		UACR:                 b.UACR,
	}
}
