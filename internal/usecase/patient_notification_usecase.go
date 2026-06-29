package usecase

import (
	"context"
	"fmt"
	"sehatiku-backend/internal/entity"
	"sehatiku-backend/internal/model"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

type patientNotifRepo interface {
	FindInAppByPatientID(db *gorm.DB, patientID string) ([]entity.Notification, error)
}

type PatientNotificationUseCase struct {
	DB        *gorm.DB
	NotifRepo patientNotifRepo
	Log       *zap.Logger
}

func (u *PatientNotificationUseCase) GetNotifications(ctx context.Context, patientID string) ([]model.PatientNotificationResponse, error) {
	rows, err := u.NotifRepo.FindInAppByPatientID(u.DB, patientID)
	if err != nil {
		return nil, fmt.Errorf("fetching in-app notifications for patient %s: %w", patientID, err)
	}

	out := make([]model.PatientNotificationResponse, len(rows))
	for i, r := range rows {
		out[i] = model.PatientNotificationResponse{
			ID:          r.ID,
			MessageType: r.MessageType,
			Payload:     r.Payload,
			CreatedAt:   r.CreatedAt,
		}
	}
	return out, nil
}
