package usecase

import (
	"context"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

// pushTokenRepo — subset of DevicePushTokenRepository needed to send + clean up. Declared
// here (not on the repository package) per be_implemantation.md §7: narrow interfaces live
// next to the usecase that mocks them.
type pushTokenRepo interface {
	FindActiveByPatient(db *gorm.DB, patientID string) ([]string, error)
	DeactivateTokens(db *gorm.DB, tokens []string) error
}

// pushSender — subset of push.PushGateway needed to send a multicast push.
type pushSender interface {
	SendMulticast(ctx context.Context, tokens []string, title, body string, data map[string]string) ([]string, error)
}

// PushUseCase adalah satu titik agregasi untuk push notification pasien: ambil token aktif,
// kirim, lalu nonaktifkan token yang FCM tandai invalid. Diinjeksikan sebagai PushNotifier
// ke usecase lain (escalation, konsultasi) — logika token/cleanup tidak diduplikasi di
// pemanggil.
type PushUseCase struct {
	DB        *gorm.DB
	TokenRepo pushTokenRepo
	Gateway   pushSender
	Log       *zap.Logger
}

// Notify mengirim push ke semua device aktif milik pasien. Best-effort: kegagalan apa pun
// hanya di-log, tidak pernah dikembalikan ke pemanggil — pola sama dengan WA fire-and-forget
// yang sudah ada di EscalationUseCase.
func (u *PushUseCase) Notify(ctx context.Context, patientID, title, body string, data map[string]string) {
	tokens, err := u.TokenRepo.FindActiveByPatient(u.DB, patientID)
	if err != nil {
		u.Log.Warn("push: gagal ambil token aktif", zap.String("patient_id", patientID), zap.Error(err))
		return
	}
	if len(tokens) == 0 {
		return
	}

	invalidTokens, err := u.Gateway.SendMulticast(ctx, tokens, title, body, data)
	if err != nil {
		u.Log.Warn("push: gagal kirim multicast", zap.String("patient_id", patientID), zap.Error(err))
		return
	}
	if len(invalidTokens) > 0 {
		if err := u.TokenRepo.DeactivateTokens(u.DB, invalidTokens); err != nil {
			u.Log.Warn("push: gagal nonaktifkan token invalid", zap.Error(err))
		}
	}
}
