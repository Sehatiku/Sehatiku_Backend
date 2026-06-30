package repository

import (
	"encoding/json"
	"fmt"
	"time"

	"gorm.io/gorm"
)

type DashboardSummaryRow struct {
	Total  int64
	Bahaya int64
	Aman   int64
}

type PatientQueueRow struct {
	ID          string
	FullName    string
	DateOfBirth *time.Time
	DiseaseType string
	Score       int
	RiskStatus  string
	TopFactors  []byte
}

type DashboardRepository struct{}

func (r *DashboardRepository) GetSummary(db *gorm.DB, faskesID string) (DashboardSummaryRow, error) {
	var result DashboardSummaryRow
	err := db.Raw(`
		WITH latest_risk AS (
			SELECT DISTINCT ON (rs.patient_id)
				rs.patient_id,
				rs.status
			FROM risk_scores rs
			INNER JOIN patients p ON rs.patient_id = p.id
			WHERE p.faskes_id = ? AND p.status = 'active'
			ORDER BY rs.patient_id, rs.scored_at DESC
		)
		SELECT
			COUNT(p.id)                                                              AS total,
			COALESCE(SUM(CASE WHEN lr.status = 'bahaya' THEN 1 ELSE 0 END), 0)     AS bahaya,
			COALESCE(SUM(CASE WHEN lr.status = 'aman'   THEN 1 ELSE 0 END), 0)     AS aman
		FROM patients p
		LEFT JOIN latest_risk lr ON p.id = lr.patient_id
		WHERE p.faskes_id = ? AND p.status = 'active'
	`, faskesID, faskesID).Scan(&result).Error
	if err != nil {
		return DashboardSummaryRow{}, fmt.Errorf("getting dashboard summary: %w", err)
	}
	return result, nil
}

func (r *DashboardRepository) GetPatientQueue(db *gorm.DB, faskesID string, limit, offset int) ([]PatientQueueRow, int64, error) {
	var rows []struct {
		ID          string
		FullName    string
		DateOfBirth *time.Time
		DiseaseType string
		Score       int
		RiskStatus  string
		TopFactors  json.RawMessage
	}

	err := db.Raw(`
		WITH latest_risk AS (
			SELECT DISTINCT ON (rs.patient_id)
				rs.patient_id,
				rs.score,
				rs.status,
				rs.top_factors
			FROM risk_scores rs
			INNER JOIN patients p ON rs.patient_id = p.id
			WHERE p.faskes_id = ? AND p.status = 'active'
			ORDER BY rs.patient_id, rs.scored_at DESC
		)
		SELECT
			p.id,
			p.full_name,
			p.date_of_birth,
			p.disease_type,
			COALESCE(lr.score, 0)       AS score,
			COALESCE(lr.status, 'aman') AS risk_status,
			lr.top_factors
		FROM patients p
		LEFT JOIN latest_risk lr ON p.id = lr.patient_id
		WHERE p.faskes_id = ? AND p.status = 'active'
		-- Triase: paling berisiko di atas. score = health_score (TINGGI = sehat), jadi
		-- urutkan status terburuk dulu, lalu health_score TERENDAH dulu dalam tiap status.
		-- Pasien tanpa skor (status NULL) ditaruh paling bawah.
		ORDER BY
			CASE COALESCE(lr.status, 'none')
				WHEN 'bahaya' THEN 0
				WHEN 'waswas' THEN 1
				WHEN 'aman'   THEN 2
				ELSE 3
			END ASC,
			COALESCE(lr.score, 1000) ASC
		LIMIT ? OFFSET ?
	`, faskesID, faskesID, limit, offset).Scan(&rows).Error
	if err != nil {
		return nil, 0, fmt.Errorf("getting patient queue: %w", err)
	}

	var total int64
	if err := db.Raw(
		`SELECT COUNT(*) FROM patients WHERE faskes_id = ? AND status = 'active'`,
		faskesID,
	).Scan(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("counting patients: %w", err)
	}

	out := make([]PatientQueueRow, len(rows))
	for i, row := range rows {
		out[i] = PatientQueueRow{
			ID:          row.ID,
			FullName:    row.FullName,
			DateOfBirth: row.DateOfBirth,
			DiseaseType: row.DiseaseType,
			Score:       row.Score,
			RiskStatus:  row.RiskStatus,
			TopFactors:  row.TopFactors,
		}
	}
	return out, total, nil
}
