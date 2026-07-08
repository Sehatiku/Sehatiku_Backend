package usecase

import (
	"context"
	"errors"
	"fmt"
	"math"
	"sehatiku-backend/internal/entity"
	"sehatiku-backend/internal/model"
	"sehatiku-backend/internal/repository"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

var ErrPatientNotFound = errors.New("pasien tidak ditemukan")

type PatientUseCase struct {
	DB            *gorm.DB
	PatientRepo   patientRepository
	NakesRepo     patientNakesLookupRepository
	BaselineRepo  patientBaselineRepo
	HistoryRepo   patientRecordHistoryRepo
	RiskScoreRepo patientRiskScoreRepo
	Log           *zap.Logger
}

type patientRepository interface {
	FindByFaskesIDWithRisk(db *gorm.DB, faskesID string, limit, offset int) ([]repository.PatientWithRisk, int64, error)
	FindByID(db *gorm.DB, id string) (*entity.Patient, error)
}

type patientNakesLookupRepository interface {
	FindByID(db *gorm.DB, id string) (*entity.Nakes, error)
}

type patientBaselineRepo interface {
	FindLatestByPatient(db *gorm.DB, patientID string) (*entity.PatientClinicalBaseline, error)
}

type patientRecordHistoryRepo interface {
	GetRecordHistory(db *gorm.DB, patientID string, limit int) ([]repository.RecordHistoryRaw, error)
}

type patientRiskScoreRepo interface {
	FindLatestByPatient(db *gorm.DB, patientID string) (*entity.RiskScore, error)
	ListByPatient(db *gorm.DB, patientID string, limit int) ([]repository.RiskScoreHistoryRow, error)
}

// ListPatients mengembalikan daftar pasien (semua status) milik faskes yang sedang
// login, dengan pagination. Setiap item menyertakan risk score terbaru pasien
// (health_score, risk_status, top_factors) — nil bila pasien belum pernah di-score.
// faskesID berasal dari JWT — tenant isolation dijaga di query repository.
func (u *PatientUseCase) ListPatients(ctx context.Context, faskesID string, page, size int) ([]model.PatientListItem, model.PageMetadata, error) {
	offset := (page - 1) * size
	patients, total, err := u.PatientRepo.FindByFaskesIDWithRisk(u.DB, faskesID, size, offset)
	if err != nil {
		return nil, model.PageMetadata{}, fmt.Errorf("listing patients: %w", err)
	}

	items := make([]model.PatientListItem, len(patients))
	for i, p := range patients {
		items[i] = model.PatientListItem{
			PatientID:      p.ID,
			FullName:       p.FullName,
			NIK:            p.NIK,
			Sex:            p.Sex,
			Age:            calcAge(p.DateOfBirth),
			DiseaseType:    p.DiseaseType,
			PhoneNumber:    p.PhoneNumber,
			CompanionName:  p.CompanionName,
			CompanionPhone: p.CompanionPhone,
			Status:         p.Status,
			EnrolledAt:     p.EnrolledAt,
			HealthScore:    p.HealthScore,
			RiskStatus:     p.RiskStatus,
			TopFactors:     p.TopFactors,
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

// GetPatientDetail mengembalikan profil lengkap satu pasien milik faskes yang sedang
// login. faskesID berasal dari JWT — pasien milik faskes lain dikembalikan sebagai
// not-found (bukan forbidden) agar keberadaannya tidak bocor lintas tenant.
func (u *PatientUseCase) GetPatientDetail(ctx context.Context, faskesID, patientID string) (*model.PatientDetailResponse, error) {
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

	// Nama nakes penanggung jawab dilampirkan untuk tampilan detail. Kegagalan lookup
	// (mis. nakes terhapus) tidak boleh menggagalkan seluruh detail pasien — nama
	// dibiarkan kosong dan dicatat sebagai warning.
	var assignedNakesName string
	if patient.AssignedNakesID != "" {
		nakes, err := u.NakesRepo.FindByID(u.DB, patient.AssignedNakesID)
		if err != nil {
			u.Log.Warn("assigned nakes not found for patient detail",
				zap.String("patient_id", patient.ID),
				zap.String("assigned_nakes_id", patient.AssignedNakesID),
				zap.Error(err),
			)
		} else {
			assignedNakesName = nakes.FullName
		}
	}

	var dob string
	if patient.DateOfBirth != nil {
		dob = patient.DateOfBirth.Format("2006-01-02")
	}

	return &model.PatientDetailResponse{
		PatientID:         patient.ID,
		FaskesID:          patient.FaskesID,
		AssignedNakesID:   patient.AssignedNakesID,
		AssignedNakesName: assignedNakesName,
		FullName:          patient.FullName,
		NIK:               patient.NIK,
		DateOfBirth:       dob,
		Sex:               patient.Sex,
		Age:               calcAge(patient.DateOfBirth),
		Alamat:            patient.Alamat,
		PhoneNumber:       patient.PhoneNumber,
		CompanionName:     patient.CompanionName,
		CompanionPhone:    patient.CompanionPhone,
		DiseaseType:       patient.DiseaseType,
		Username:          patient.Username,
		Status:            patient.Status,
		EnrolledAt:        patient.EnrolledAt,
		CreatedAt:         patient.CreatedAt,
		UpdatedAt:         patient.UpdatedAt,
	}, nil
}

// GetNakesPatientDetail mengembalikan detail pasien untuk dokter/nakes,
// termasuk history baseline, log harian, dan atribut risiko.
func (u *PatientUseCase) GetNakesPatientDetail(ctx context.Context, faskesID, nakesID, patientID string) (*model.NakesPatientDetailResponse, error) {
	// Re-use GetPatientDetail untuk cek tenant isolation & ambil profil dasar.
	patientDetail, err := u.GetPatientDetail(ctx, faskesID, patientID)
	if err != nil {
		return nil, err
	}

	var baselineDetail *model.BaselineDetailResponse
	baseline, err := u.BaselineRepo.FindLatestByPatient(u.DB, patientID)
	if err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			u.Log.Error("failed to fetch patient baseline", zap.Error(err))
		}
	} else {
		var recordedByName string
		if baseline.RecordedByNakesID != nil {
			n, err := u.NakesRepo.FindByID(u.DB, *baseline.RecordedByNakesID)
			if err == nil {
				recordedByName = n.FullName
			}
		}
		baselineDetail = &model.BaselineDetailResponse{
			ID:                    baseline.ID,
			PatientID:             baseline.PatientID,
			RecordedAt:            baseline.RecordedAt,
			RecordedByNakesID:     baseline.RecordedByNakesID,
			RecordedByNakesName:   recordedByName,
			Notes:                 baseline.Notes,
			AgeYears:              baseline.AgeYears,
			Sex:                   baseline.Sex,
			BMI:                   baseline.BMI,
			BMICategory:           baseline.BMICategory,
			WaistCircumferenceCm:  baseline.WaistCircumferenceCm,
			CentralObesity:        baseline.CentralObesity,
			SmokingStatus:         baseline.SmokingStatus,
			AlcoholUse:            baseline.AlcoholUse,
			PhysicalActivity:      baseline.PhysicalActivity,
			FamilyHistoryDiabetes: baseline.FamilyHistoryDiabetes,
			FamilyHistoryCVD:      baseline.FamilyHistoryCVD,
			SystolicBPMmhg:        baseline.SystolicBPMmhg,
			DiastolicBPMmhg:       baseline.DiastolicBPMmhg,
			HypertensionStatus:    baseline.HypertensionStatus,
			FastingGlucoseMgdl:    baseline.FastingGlucoseMgdl,
			HbA1cPct:              baseline.HbA1cPct,
			DiabetesStatus:        baseline.DiabetesStatus,
			TotalCholesterolMgdl:  baseline.TotalCholesterolMgdl,
			HDLMgdl:               baseline.HDLMgdl,
			LDLMgdl:               baseline.LDLMgdl,
			TriglyceidesMgdl:      baseline.TriglyceidesMgdl,
			CVDRisk10YrPct:        baseline.CVDRisk10YrPct,
			CVDRiskCategory:       baseline.CVDRiskCategory,
			OnAntihypertensive:    baseline.OnAntihypertensive,
			OnAntidiabetic:        baseline.OnAntidiabetic,
			OnStatin:              baseline.OnStatin,
			TargetRisk:            baseline.TargetRisk,
			EGFR:                  baseline.EGFR,
			UACR:                  baseline.UACR,
			ClusterID:             baseline.ClusterID,
			DiagnosisCluster:      baseline.DiagnosisCluster,
			ClinicalGroup:         baseline.ClinicalGroup,
		}
	}

	historyRows, err := u.HistoryRepo.GetRecordHistory(u.DB, patientID, 7) // 7 days of daily logs
	var dailyLogs []model.RecordHistoryItem
	if err == nil {
		dailyLogs = mapRecordHistoryRows(historyRows)
	}

	var riskFactorStatus *model.PatientRiskFactorStatus
	riskScore, err := u.RiskScoreRepo.FindLatestByPatient(u.DB, patientID)
	if err == nil && riskScore != nil {
		riskFactorStatus = &model.PatientRiskFactorStatus{
			Score:       riskScore.Score,
			Status:      riskScore.Status,
			ScoringMode: riskScore.ScoringMode,
			TopFactors:  riskScore.TopFactors,
		}
	}

	var healthScoreHistory []model.HealthScorePoint
	scores, err := u.RiskScoreRepo.ListByPatient(u.DB, patientID, 7) // up to 7 latest history
	if err == nil {
		for _, s := range scores {
			healthScoreHistory = append(healthScoreHistory, model.HealthScorePoint{
				Score:    s.Score,
				Status:   s.Status,
				ScoredAt: s.ScoredAt,
			})
		}
	}

	return &model.NakesPatientDetailResponse{
		PatientDetail:      *patientDetail,
		Baseline:           baselineDetail,
		DailyLogs:          dailyLogs,
		Risk:               riskFactorStatus,
		HealthScoreHistory: healthScoreHistory,
	}, nil
}
