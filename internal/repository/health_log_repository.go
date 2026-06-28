package repository

import (
	"fmt"
	"sehatiku-backend/internal/entity"

	"gorm.io/gorm"
)

// HealthLogRepository menangani penulisan ke tabel health_logs.
// Tabel ini insert-only (full audit trail data medis, lihat docs/erd.md), karena itu
// repository ini SENGAJA tidak meng-embed Repository[T] generic — tidak ada Update/Delete
// yang boleh menyentuh baris health_logs.
type HealthLogRepository struct{}

func (r *HealthLogRepository) Create(db *gorm.DB, log *entity.HealthLog) error {
	if err := db.Create(log).Error; err != nil {
		return fmt.Errorf("creating health log: %w", err)
	}
	return nil
}

func (r *HealthLogRepository) FindByID(db *gorm.DB, id string) (*entity.HealthLog, error) {
	var log entity.HealthLog
	if err := db.Where("id = ?", id).First(&log).Error; err != nil {
		return nil, fmt.Errorf("finding health log by id: %w", err)
	}
	return &log, nil
}
