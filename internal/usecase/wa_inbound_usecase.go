package usecase

import (
	"context"
	"fmt"

	"sehatiku-backend/internal/entity"
	"sehatiku-backend/internal/helper"
	"sehatiku-backend/internal/repository"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

// pendingCredentialStore membaca & menghapus kredensial yang menunggu warm-up di Redis.
type pendingCredentialStore interface {
	Get(ctx context.Context, phone string) (*repository.PendingCredential, error)
	Delete(ctx context.Context, phone string) error
}

// warmupSender mengirim kredensial ke kontak yang sudah hangat (sudah menghubungi bot).
// Cukup dua method gateway WA yang sudah ada — di-interface-kan agar usecase bisa diuji
// dengan mock (be_implementation §7).
type warmupSender interface {
	SendRegistrationCredentials(ctx context.Context, toPhone, recipientName, username, password string) error
	SendCompanionRegistrationCredentials(ctx context.Context, toPhone, companionName, patientName, username, password string) error
}

// notificationStatusUpdater memperbarui status baris audit notifications.
type notificationStatusUpdater interface {
	MarkStatus(db *gorm.DB, id, status string, errReason *string) error
}

// WAInboundUseCase menangani pesan WA masuk untuk alur warm-up: saat penerima (pasien/
// pendamping/nakes) menghubungi bot lebih dulu, percakapan menjadi hangat sehingga bot
// bisa mengirim kredensial tanpa kena blok kontak-dingin (error 463). Usecase ini mencari
// kredensial yang menunggu untuk nomor pengirim, mengirimnya, lalu membersihkan stash.
type WAInboundUseCase struct {
	DB                *gorm.DB
	PendingCredential pendingCredentialStore
	WhatsApp          warmupSender
	NotificationRepo  notificationStatusUpdater
	Log               *zap.Logger
}

// DeliverPendingCredential mencari kredensial menunggu pada salah satu kandidat nomor
// pengirim (Sender bisa beralamat LID, SenderAlt nomor telepon — keduanya dicoba), lalu
// mengirimkannya. Tidak adanya kredensial menunggu adalah kondisi normal (mayoritas pesan
// masuk bukan warm-up) dan dikembalikan sebagai nil, bukan error.
func (u *WAInboundUseCase) DeliverPendingCredential(ctx context.Context, candidatePhones []string) error {
	for _, phone := range candidatePhones {
		if phone == "" {
			continue
		}
		pending, err := u.PendingCredential.Get(ctx, phone)
		if err != nil {
			return fmt.Errorf("looking up pending credential for %s: %w", helper.MaskPhone(phone), err)
		}
		if pending == nil {
			continue
		}
		return u.deliver(ctx, phone, pending)
	}
	return nil
}

func (u *WAInboundUseCase) deliver(ctx context.Context, phone string, p *repository.PendingCredential) error {
	var sendErr error
	switch p.Role {
	case entity.RecipientRoleCompanion:
		sendErr = u.WhatsApp.SendCompanionRegistrationCredentials(ctx, phone, p.RecipientName, p.PatientName, p.Username, p.Password)
	default: // patient | nakes — pesan kredensial yang sama
		sendErr = u.WhatsApp.SendRegistrationCredentials(ctx, phone, p.RecipientName, p.Username, p.Password)
	}

	if sendErr != nil {
		// Stash sengaja TIDAK dihapus: kontak sudah hangat, jadi percobaan berikutnya
		// (pesan masuk lain dari nomor ini, atau retry) berpeluang berhasil.
		u.markNotification(p.NotificationID, entity.NotificationStatusFailed, sendErr)
		return fmt.Errorf("sending warm-up credential to %s: %w", helper.MaskPhone(phone), sendErr)
	}

	u.markNotification(p.NotificationID, entity.NotificationStatusSent, nil)
	if err := u.PendingCredential.Delete(ctx, phone); err != nil {
		u.Log.Warn("failed to delete pending credential after send",
			zap.String("phone", helper.MaskPhone(phone)), zap.Error(err))
	}
	u.Log.Info("warm-up credential delivered",
		zap.String("role", p.Role), zap.String("phone", helper.MaskPhone(phone)))
	return nil
}

func (u *WAInboundUseCase) markNotification(id, status string, sendErr error) {
	if id == "" {
		return
	}
	var reason *string
	if sendErr != nil {
		r := sendErr.Error()
		reason = &r
	}
	if err := u.NotificationRepo.MarkStatus(u.DB, id, status, reason); err != nil {
		u.Log.Warn("failed to update notification status",
			zap.String("notification_id", id), zap.String("status", status), zap.Error(err))
	}
}
