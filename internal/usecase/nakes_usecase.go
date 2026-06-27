package usecase

import (
	"context"
	"fmt"
	"sehatiku-backend/internal/entity"
	"sehatiku-backend/internal/model"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

type NakesUseCase struct {
	DB        *gorm.DB
	NakesRepo nakesLister
	Log       *zap.Logger
}

type nakesLister interface {
	FindByFaskesID(db *gorm.DB, faskesID string) ([]entity.Nakes, error)
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
