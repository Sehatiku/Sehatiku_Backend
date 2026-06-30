package repository

import (
	"errors"
	"fmt"
	"sehatiku-backend/internal/entity"
	"sehatiku-backend/internal/model"
	"time"

	"gorm.io/gorm"
)

// EscalationRepository menangani tabel `escalations` (mutable: status & feedback berkembang).
type EscalationRepository struct{}

func (r *EscalationRepository) Create(db *gorm.DB, e *entity.Escalation) error {
	if err := db.Create(e).Error; err != nil {
		return fmt.Errorf("creating escalation: %w", err)
	}
	return nil
}

// FindActiveByPatientTier mengembalikan eskalasi yang masih terbuka (status sent/viewed)
// untuk pasien+tier tertentu, atau (nil, nil) bila tidak ada. Dipakai untuk dedup.
func (r *EscalationRepository) FindActiveByPatientTier(db *gorm.DB, patientID, tier string) (*entity.Escalation, error) {
	var e entity.Escalation
	err := db.
		Where("patient_id = ? AND tier = ? AND status IN ?",
			patientID, tier,
			[]string{entity.EscalationStatusSent, entity.EscalationStatusViewed}).
		Order("sent_at DESC").
		First(&e).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("finding active escalation for patient %s: %w", patientID, err)
	}
	return &e, nil
}

func (r *EscalationRepository) FindByID(db *gorm.DB, id string) (*entity.Escalation, error) {
	var e entity.Escalation
	if err := db.Where("id = ?", id).First(&e).Error; err != nil {
		return nil, fmt.Errorf("finding escalation %s: %w", id, err)
	}
	return &e, nil
}

// UpdateStatus mengubah status eskalasi dan menstempel timestamp lifecycle yang sesuai.
func (r *EscalationRepository) UpdateStatus(db *gorm.DB, id, status string, at time.Time) error {
	updates := map[string]any{"status": status, "updated_at": at}
	switch status {
	case entity.EscalationStatusViewed:
		updates["viewed_at"] = at
	case entity.EscalationStatusActed:
		updates["acted_at"] = at
	}
	res := db.Model(&entity.Escalation{}).Where("id = ?", id).Updates(updates)
	if res.Error != nil {
		return fmt.Errorf("updating escalation %s status: %w", id, res.Error)
	}
	if res.RowsAffected == 0 {
		return fmt.Errorf("escalation %s not found: %w", id, gorm.ErrRecordNotFound)
	}
	return nil
}

// FindByFaskes mengembalikan satu halaman eskalasi milik faskes, acute lebih dulu lalu
// terbaru. status/tier kosong = tanpa filter. Di-JOIN dengan patients & risk_scores.
func (r *EscalationRepository) FindByFaskes(db *gorm.DB, faskesID, status, tier string, limit, offset int) ([]model.EscalationQueueItem, int64, error) {
	where := "e.faskes_id = ?"
	args := []any{faskesID}
	if status != "" {
		where += " AND e.status = ?"
		args = append(args, status)
	}
	if tier != "" {
		where += " AND e.tier = ?"
		args = append(args, tier)
	}

	listArgs := append(append([]any{}, args...), limit, offset)
	var items []model.EscalationQueueItem
	err := db.Raw(`
		SELECT
			e.id, e.patient_id, p.full_name AS patient_name,
			e.tier, e.status,
			COALESCE(rs.score, 0)   AS risk_score,
			COALESCE(rs.status, '') AS risk_status,
			e.sent_at, e.viewed_at, e.acted_at, e.created_at
		FROM escalations e
		JOIN patients p ON p.id = e.patient_id
		LEFT JOIN risk_scores rs ON rs.id = e.risk_score_id
		WHERE `+where+`
		ORDER BY (e.tier = 'acute_today') DESC, e.sent_at DESC
		LIMIT ? OFFSET ?
	`, listArgs...).Scan(&items).Error
	if err != nil {
		return nil, 0, fmt.Errorf("listing escalations for faskes %s: %w", faskesID, err)
	}

	var total int64
	if err := db.Raw(`SELECT COUNT(*) FROM escalations e WHERE `+where, args...).Scan(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("counting escalations for faskes %s: %w", faskesID, err)
	}
	return items, total, nil
}
