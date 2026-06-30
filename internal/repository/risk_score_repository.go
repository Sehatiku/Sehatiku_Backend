package repository

import (
	"fmt"
	"sehatiku-backend/internal/entity"
	"time"

	"gorm.io/gorm"
)

// RiskScoreRepository writes ML scoring results to risk_scores (insert-only audit row).
type RiskScoreRepository struct{}

func (r *RiskScoreRepository) Create(db *gorm.DB, score *entity.RiskScore) error {
	if err := db.Create(score).Error; err != nil {
		return fmt.Errorf("creating risk score: %w", err)
	}
	return nil
}

// RiskScoreHistoryRow adalah satu titik tren health score (score 0-100 + status) pada waktu tertentu.
type RiskScoreHistoryRow struct {
	Score    int       `gorm:"column:score"`
	Status   string    `gorm:"column:status"`
	ScoredAt time.Time `gorm:"column:scored_at"`
}

// ListByPatient mengembalikan tren health score pasien (score, status, scored_at),
// terbaru-dulu, dibatasi limit. Dipakai untuk menampilkan progress health score.
func (r *RiskScoreRepository) ListByPatient(db *gorm.DB, patientID string, limit int) ([]RiskScoreHistoryRow, error) {
	var rows []RiskScoreHistoryRow
	err := db.Raw(`
		SELECT score, status, scored_at
		FROM risk_scores
		WHERE patient_id = ?
		ORDER BY scored_at DESC
		LIMIT ?
	`, patientID, limit).Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("listing risk score history: %w", err)
	}
	return rows, nil
}

// FindLatestStatus returns the status of the most recent risk score for a patient,
// excluding excludeID (the just-created row). found=false when the patient has no other
// score yet. Used to detect a fresh transition into 'bahaya'.
func (r *RiskScoreRepository) FindLatestStatus(db *gorm.DB, patientID, excludeID string) (string, bool, error) {
	var status string
	err := db.Raw(`
		SELECT status
		FROM risk_scores
		WHERE patient_id = ? AND id <> ?
		ORDER BY scored_at DESC
		LIMIT 1
	`, patientID, excludeID).Scan(&status).Error
	if err != nil {
		return "", false, fmt.Errorf("finding latest risk status for patient %s: %w", patientID, err)
	}
	if status == "" {
		return "", false, nil
	}
	return status, true, nil
}
