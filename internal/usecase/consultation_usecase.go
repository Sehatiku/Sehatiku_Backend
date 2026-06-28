package usecase

import (
	"context"
	"fmt"
	"sehatiku-backend/internal/entity"
	"sehatiku-backend/internal/model"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

type consultationRepo interface {
	Create(db *gorm.DB, c *entity.Consultation) error
}

type ConsultationUseCase struct {
	DB   *gorm.DB
	Repo consultationRepo
	Log  *zap.Logger
}

func (u *ConsultationUseCase) CreateConsultation(ctx context.Context, patientID string, req *model.CreateConsultationRequest) (*model.ConsultationResponse, error) {
	c := &entity.Consultation{
		PatientID: patientID,
		Complaint: req.Complaint,
		Status:    "open",
	}
	if err := u.Repo.Create(u.DB, c); err != nil {
		return nil, fmt.Errorf("creating consultation for patient %s: %w", patientID, err)
	}

	u.Log.Info("consultation created",
		zap.String("patient_id", patientID),
		zap.String("consultation_id", c.ID),
	)

	return &model.ConsultationResponse{
		ID:        c.ID,
		PatientID: c.PatientID,
		Complaint: c.Complaint,
		Status:    c.Status,
		CreatedAt: c.CreatedAt,
	}, nil
}
