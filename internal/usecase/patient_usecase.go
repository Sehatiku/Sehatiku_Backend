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
	DB          *gorm.DB
	PatientRepo patientRepository
	NakesRepo   patientNakesLookupRepository
	Log         *zap.Logger
}

type patientRepository interface {
	FindByFaskesIDWithRisk(db *gorm.DB, faskesID string, limit, offset int) ([]repository.PatientWithRisk, int64, error)
	FindByID(db *gorm.DB, id string) (*entity.Patient, error)
}

type patientNakesLookupRepository interface {
	FindByID(db *gorm.DB, id string) (*entity.Nakes, error)
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
