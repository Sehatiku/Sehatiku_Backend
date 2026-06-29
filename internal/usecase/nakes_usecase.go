package usecase

import (
	"context"
	"errors"
	"fmt"
	"sehatiku-backend/internal/entity"
	"sehatiku-backend/internal/model"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

var ErrNakesNotFound = errors.New("nakes tidak ditemukan")

type NakesUseCase struct {
	DB        *gorm.DB
	NakesRepo nakesRepository
	Log       *zap.Logger
}

type nakesRepository interface {
	FindByFaskesID(db *gorm.DB, faskesID string) ([]entity.Nakes, error)
	FindByID(db *gorm.DB, id string) (*entity.Nakes, error)
	Update(db *gorm.DB, nakes *entity.Nakes) error
}

func (u *NakesUseCase) ListNakes(ctx context.Context, faskesID string) ([]model.NakesListItem, error) {
	nakesList, err := u.NakesRepo.FindByFaskesID(u.DB, faskesID)
	if err != nil {
		return nil, fmt.Errorf("listing nakes: %w", err)
	}

	items := make([]model.NakesListItem, len(nakesList))
	for i, n := range nakesList {
		items[i] = model.NakesListItem{
			NakesID:     n.ID,
			FullName:    n.FullName,
			Role:        n.Role,
			Username:    n.Username,
			PhoneNumber: n.PhoneNumber,
			Status:      n.Status,
			EnrolledAt:  n.EnrolledAt,
		}
	}
	return items, nil
}

// GetNakesDetail mengembalikan profil lengkap satu nakes milik faskes yang sedang
// login. faskesID berasal dari JWT — nakes milik faskes lain dikembalikan sebagai
// not-found (bukan forbidden) agar keberadaannya tidak bocor lintas tenant.
func (u *NakesUseCase) GetNakesDetail(ctx context.Context, faskesID, nakesID string) (*model.NakesDetailResponse, error) {
	nakes, err := u.NakesRepo.FindByID(u.DB, nakesID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNakesNotFound
		}
		return nil, fmt.Errorf("finding nakes %s: %w", nakesID, err)
	}

	if nakes.FaskesID != faskesID {
		return nil, ErrNakesNotFound
	}

	return &model.NakesDetailResponse{
		NakesID:     nakes.ID,
		FaskesID:    nakes.FaskesID,
		FullName:    nakes.FullName,
		Role:        nakes.Role,
		NIK:         nakes.NIK,
		Alamat:      nakes.Alamat,
		PhoneNumber: nakes.PhoneNumber,
		Username:    nakes.Username,
		Status:      nakes.Status,
		EnrolledAt:  nakes.EnrolledAt,
		CreatedAt:   nakes.CreatedAt,
		UpdatedAt:   nakes.UpdatedAt,
	}, nil
}

func (u *NakesUseCase) GetMyProfile(ctx context.Context, nakesID string) (*model.NakesDetailResponse, error) {
	nakes, err := u.NakesRepo.FindByID(u.DB, nakesID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNakesNotFound
		}
		return nil, fmt.Errorf("finding nakes %s: %w", nakesID, err)
	}

	return &model.NakesDetailResponse{
		NakesID:     nakes.ID,
		FaskesID:    nakes.FaskesID,
		FullName:    nakes.FullName,
		Role:        nakes.Role,
		NIK:         nakes.NIK,
		Alamat:      nakes.Alamat,
		PhoneNumber: nakes.PhoneNumber,
		Username:    nakes.Username,
		Status:      nakes.Status,
		EnrolledAt:  nakes.EnrolledAt,
		CreatedAt:   nakes.CreatedAt,
		UpdatedAt:   nakes.UpdatedAt,
	}, nil
}

func (u *NakesUseCase) UpdateNakesStatus(ctx context.Context, faskesID, nakesID string, req *model.UpdateNakesStatusRequest) (*model.UpdateNakesStatusResponse, error) {
	nakes, err := u.NakesRepo.FindByID(u.DB, nakesID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrNakesNotFound
		}
		return nil, fmt.Errorf("finding nakes %s: %w", nakesID, err)
	}

	// Isolasi tenant: faskes hanya boleh mengubah nakes miliknya sendiri. Kembalikan
	// not-found (bukan forbidden) agar keberadaan nakes milik faskes lain tidak bocor.
	if nakes.FaskesID != faskesID {
		return nil, ErrNakesNotFound
	}

	nakes.Status = req.Status
	if err := u.NakesRepo.Update(u.DB, nakes); err != nil {
		return nil, fmt.Errorf("updating nakes %s status: %w", nakesID, err)
	}

	u.Log.Info("nakes status updated",
		zap.String("nakes_id", nakes.ID),
		zap.String("faskes_id", faskesID),
		zap.String("status", nakes.Status),
	)

	return &model.UpdateNakesStatusResponse{
		NakesID:  nakes.ID,
		FullName: nakes.FullName,
		Status:   nakes.Status,
	}, nil
}
