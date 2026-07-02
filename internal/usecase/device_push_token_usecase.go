package usecase

import (
	"context"
	"fmt"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

// devicePushTokenRepo — subset of DevicePushTokenRepository needed for register/deregister.
type devicePushTokenRepo interface {
	Upsert(db *gorm.DB, patientID, platform, token string) error
	DeactivateByToken(db *gorm.DB, patientID, token string) error
}

// DevicePushTokenUseCase menangani registrasi/deregistrasi token FCM milik pasien.
type DevicePushTokenUseCase struct {
	DB   *gorm.DB
	Repo devicePushTokenRepo
	Log  *zap.Logger
}

// Register mendaftarkan/memperbarui satu device token milik pasien (upsert by token).
func (u *DevicePushTokenUseCase) Register(ctx context.Context, patientID, token, platform string) error {
	if err := u.Repo.Upsert(u.DB, patientID, platform, token); err != nil {
		return fmt.Errorf("registering device token for patient %s: %w", patientID, err)
	}
	u.Log.Info("device push token registered", zap.String("patient_id", patientID), zap.String("platform", platform))
	return nil
}

// Deregister menonaktifkan satu device token milik pasien (dipanggil saat logout).
func (u *DevicePushTokenUseCase) Deregister(ctx context.Context, patientID, token string) error {
	if err := u.Repo.DeactivateByToken(u.DB, patientID, token); err != nil {
		return fmt.Errorf("deregistering device token for patient %s: %w", patientID, err)
	}
	u.Log.Info("device push token deregistered", zap.String("patient_id", patientID))
	return nil
}
