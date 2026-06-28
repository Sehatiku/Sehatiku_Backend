package repository

import (
	"sehatiku-backend/internal/entity"
)

// NotificationRepository menyimpan catatan pesan keluar (audit transport WA/SMS).
// Memakai operasi generik Create/Update dari Repository[T]; belum ada query khusus
// karena retry worker / endpoint listing belum dibuat (lihat docs erd: kolom
// status & retry_count sudah siap bila nanti dibutuhkan).
type NotificationRepository struct {
	Repository[entity.Notification]
}
