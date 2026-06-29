package repository

import (
	"time"

	"sehatiku-backend/internal/entity"

	"gorm.io/gorm"
)

// NotificationRepository menyimpan catatan pesan keluar (audit transport WA/SMS murni:
// credential_blast, daily_prompt, escalation, recommendation, system). Inbox in-app pasien
// TIDAK lagi di sini — sudah dipindah ke PatientNotificationRepository (tabel
// patient_notifications). Memakai operasi generik Create/Update dari Repository[T].
type NotificationRepository struct {
	Repository[entity.Notification]
}

// MarkStatus memperbarui status satu baris notifikasi berdasarkan id tanpa harus memuat
// seluruh entity dulu. Saat status "sent", sent_at ikut diisi; error_reason diisi bila ada.
func (r *NotificationRepository) MarkStatus(db *gorm.DB, id, status string, errReason *string) error {
	updates := map[string]any{
		"status":     status,
		"updated_at": time.Now(),
	}
	if status == entity.NotificationStatusSent {
		updates["sent_at"] = time.Now()
	}
	if errReason != nil {
		updates["error_reason"] = *errReason
	}
	return db.Model(&entity.Notification{}).Where("id = ?", id).Updates(updates).Error
}
