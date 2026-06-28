package repository

import (
	"time"

	"sehatiku-backend/internal/entity"

	"gorm.io/gorm"
)

// NotificationRepository menyimpan catatan pesan keluar (audit transport WA/SMS).
// Memakai operasi generik Create/Update dari Repository[T]; belum ada query khusus
// karena retry worker / endpoint listing belum dibuat (lihat docs erd: kolom
// status & retry_count sudah siap bila nanti dibutuhkan).
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
