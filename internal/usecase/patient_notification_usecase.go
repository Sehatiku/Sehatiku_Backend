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
	FindByPatientID(db *gorm.DB, patientID string) ([]entity.PatientNotification, error)
	CountUnread(db *gorm.DB, patientID string) (int64, error)
	MarkRead(db *gorm.DB, id, patientID string) (int64, error)
	ExistsForPatient(db *gorm.DB, id, patientID string) (bool, error)
	MarkAllRead(db *gorm.DB, patientID string) (int64, error)
}

type PatientNotificationUseCase struct {
	DB        *gorm.DB
	NotifRepo patientNotifRepo
	Log       *zap.Logger
}

// notifPayload adalah bentuk JSONB yang disimpan di patient_notifications.payload.
// nakes_name dipakai consultation_reply; field lain bisa ditambah per-tipe tanpa migrasi.
type notifPayload struct {
	NakesName string `json:"nakes_name"`
}

func (u *PatientNotificationUseCase) GetNotifications(ctx context.Context, patientID string) ([]model.PatientNotificationResponse, error) {
	rows, err := u.NotifRepo.FindByPatientID(u.DB, patientID)
	if err != nil {
		return nil, fmt.Errorf("fetching notifications for patient %s: %w", patientID, err)
	}

	out := make([]model.PatientNotificationResponse, len(rows))
	for i, r := range rows {
		item := model.PatientNotificationResponse{
			ID:        r.ID,
			Type:      r.Type,
			Title:     r.Title,
			Body:      r.Body,
			IsRead:    r.ReadAt != nil,
			ReadAt:    r.ReadAt,
			CreatedAt: r.CreatedAt,
			Data: model.PatientNotificationData{
				ConsultationID: r.ConsultationID,
			},
		}
		if r.Payload != nil && *r.Payload != "" {
			var p notifPayload
			if err := json.Unmarshal([]byte(*r.Payload), &p); err != nil {
				u.Log.Warn("failed to parse patient notification payload",
					zap.String("notification_id", r.ID),
					zap.Error(err),
				)
			} else if p.NakesName != "" {
				name := p.NakesName
				item.Data.NakesName = &name
			}
		}
		out[i] = item
	}
	return out, nil
}

func (u *PatientNotificationUseCase) GetUnreadCount(ctx context.Context, patientID string) (*model.UnreadCountResponse, error) {
	count, err := u.NotifRepo.CountUnread(u.DB, patientID)
	if err != nil {
		return nil, fmt.Errorf("counting unread notifications for patient %s: %w", patientID, err)
	}
	return &model.UnreadCountResponse{UnreadCount: count}, nil
}

// MarkRead menandai satu notifikasi sebagai terbaca. Idempoten: jika notifikasi memang milik
// pasien tapi sudah terbaca, tetap sukses (no-op). Notifikasi yang tidak ada / milik pasien
// lain dikembalikan sebagai error "not found" (404 di controller, tidak bocor lintas pasien).
func (u *PatientNotificationUseCase) MarkRead(ctx context.Context, id, patientID string) error {
	affected, err := u.NotifRepo.MarkRead(u.DB, id, patientID)
	if err != nil {
		return fmt.Errorf("marking notification %s read: %w", id, err)
	}
	if affected > 0 {
		return nil
	}

	// affected==0 bisa berarti (a) sudah terbaca, atau (b) tidak ada / bukan milik pasien.
	exists, err := u.NotifRepo.ExistsForPatient(u.DB, id, patientID)
	if err != nil {
		return fmt.Errorf("verifying notification %s ownership: %w", id, err)
	}
	if !exists {
		return fmt.Errorf("notification %s not found", id)
	}
	return nil // sudah terbaca sebelumnya — idempoten
}

func (u *PatientNotificationUseCase) MarkAllRead(ctx context.Context, patientID string) (*model.MarkAllReadResponse, error) {
	updated, err := u.NotifRepo.MarkAllRead(u.DB, patientID)
	if err != nil {
		return nil, fmt.Errorf("marking all notifications read for patient %s: %w", patientID, err)
	}
	return &model.MarkAllReadResponse{UpdatedCount: updated}, nil
}
