package usecase

import (
	"context"
	"encoding/json"
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

type consultationReplyPayload struct {
	ConsultationID string `json:"consultation_id"`
	NakesName      string `json:"nakes_name"`
	NakesNote      string `json:"nakes_note"`
}

func (u *PatientNotificationUseCase) GetNotifications(ctx context.Context, patientID string) ([]model.PatientNotificationResponse, error) {
	rows, err := u.NotifRepo.FindInAppByPatientID(u.DB, patientID)
	if err != nil {
		return nil, fmt.Errorf("fetching in-app notifications for patient %s: %w", patientID, err)
	}

	out := make([]model.PatientNotificationResponse, len(rows))
	for i, r := range rows {
		item := model.PatientNotificationResponse{
			ID:          r.ID,
			MessageType: r.MessageType,
			CreatedAt:   r.CreatedAt,
		}
		var p consultationReplyPayload
		if err := json.Unmarshal([]byte(r.Payload), &p); err == nil {
			item.ConsultationID = p.ConsultationID
			item.NakesName = p.NakesName
			item.NakesNote = p.NakesNote
		} else {
			u.Log.Warn("failed to parse notification payload",
				zap.String("notification_id", r.ID),
				zap.Error(err),
			)
		}
		out[i] = item
	}
	return out, nil
}
