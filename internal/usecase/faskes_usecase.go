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

var ErrFaskesNotFound = errors.New("faskes tidak ditemukan")

type FaskesUseCase struct {
	DB         *gorm.DB
	FaskesRepo faskesProfileRepository
	Log        *zap.Logger
}

type faskesProfileRepository interface {
	FindByID(db *gorm.DB, id string) (*entity.Faskes, error)
}

// GetFaskesProfile mengembalikan profil faskes yang sedang login. faskesID berasal
// dari JWT, bukan dari request — faskes hanya bisa melihat profilnya sendiri.
func (u *FaskesUseCase) GetFaskesProfile(ctx context.Context, faskesID string) (*model.FaskesProfileResponse, error) {
	faskes, err := u.FaskesRepo.FindByID(u.DB, faskesID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrFaskesNotFound
		}
		return nil, fmt.Errorf("finding faskes %s: %w", faskesID, err)
	}

	return &model.FaskesProfileResponse{
		FaskesID:    faskes.ID,
		Name:        faskes.Name,
		Type:        faskes.Type,
		Address:     faskes.Address,
		Region:      faskes.Region,
		Username:    faskes.Username,
		PhoneNumber: faskes.PhoneNumber,
		Status:      faskes.Status,
		CreatedAt:   faskes.CreatedAt,
		UpdatedAt:   faskes.UpdatedAt,
	}, nil
}
