package usecase

import (
	"context"
	"fmt"
	"math"
	"sehatiku-backend/internal/entity"
	"sehatiku-backend/internal/model"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

type PatientUseCase struct {
	DB          *gorm.DB
	PatientRepo patientListRepository
	Log         *zap.Logger
}

type patientListRepository interface {
	FindByFaskesID(db *gorm.DB, faskesID string, limit, offset int) ([]entity.Patient, int64, error)
}

// ListPatients mengembalikan daftar pasien (semua status) milik faskes yang sedang
// login, dengan pagination. faskesID berasal dari JWT — tenant isolation dijaga di
// query repository (WHERE faskes_id = ?), bukan dari input klien.
func (u *PatientUseCase) ListPatients(ctx context.Context, faskesID string, page, size int) ([]model.PatientListItem, model.PageMetadata, error) {
	offset := (page - 1) * size
	patients, total, err := u.PatientRepo.FindByFaskesID(u.DB, faskesID, size, offset)
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
