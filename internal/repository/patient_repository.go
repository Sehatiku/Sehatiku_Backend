package repository

import (
	"encoding/json"
	"fmt"
	"sehatiku-backend/internal/entity"

	"gorm.io/gorm"
)

// PatientWithRisk adalah hasil JOIN patients + risk_scores terbaru.
// HealthScore, RiskStatus, TopFactors bisa nil/null bila pasien belum pernah di-score.
type PatientWithRisk struct {
	entity.Patient
	HealthScore *int            `gorm:"column:health_score"`
	RiskStatus  *string         `gorm:"column:risk_status"`
	TopFactors  json.RawMessage `gorm:"column:top_factors"`
}

type PatientRepository struct {
	Repository[entity.Patient]
}

func (r *PatientRepository) FindByUsername(db *gorm.DB, username string) (*entity.Patient, error) {
	var patient entity.Patient
	if err := db.Where("username = ?", username).First(&patient).Error; err != nil {
		return nil, fmt.Errorf("finding patient by username: %w", err)
	}
	return &patient, nil
}

func (r *PatientRepository) FindByNIK(db *gorm.DB, nik string) (*entity.Patient, error) {
	var patient entity.Patient
	if err := db.Where("nik = ?", nik).First(&patient).Error; err != nil {
		return nil, fmt.Errorf("finding patient by nik: %w", err)
	}
	return &patient, nil
}

func (r *PatientRepository) FindByID(db *gorm.DB, id string) (*entity.Patient, error) {
	var patient entity.Patient
	if err := db.Where("id = ?", id).First(&patient).Error; err != nil {
		return nil, fmt.Errorf("finding patient by id: %w", err)
	}
	return &patient, nil
}

// FindByFaskesID mengembalikan satu halaman pasien milik faskes (semua status),
// diurutkan enrolled_at DESC, beserta total seluruh pasien faskes untuk pagination.
func (r *PatientRepository) FindByFaskesID(db *gorm.DB, faskesID string, limit, offset int) ([]entity.Patient, int64, error) {
	var total int64
	if err := db.Model(&entity.Patient{}).Where("faskes_id = ?", faskesID).Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("counting patients by faskes_id: %w", err)
	}

	var patients []entity.Patient
	if err := db.Where("faskes_id = ?", faskesID).
		Order("enrolled_at DESC").
		Limit(limit).Offset(offset).
		Find(&patients).Error; err != nil {
		return nil, 0, fmt.Errorf("finding patients by faskes_id: %w", err)
	}
	return patients, total, nil
}

// FindByFaskesIDWithRisk mengembalikan satu halaman pasien milik faskes beserta
// risk score terbaru masing-masing pasien (health_score, risk_status, top_factors).
// Pasien yang belum pernah di-score tetap muncul dengan nilai nil pada ketiga field tsb.
// Menggunakan CTE + DISTINCT ON (pola sama dengan DashboardRepository) untuk efisiensi.
func (r *PatientRepository) FindByFaskesIDWithRisk(db *gorm.DB, faskesID string, limit, offset int) ([]PatientWithRisk, int64, error) {
	var total int64
	if err := db.Model(&entity.Patient{}).Where("faskes_id = ?", faskesID).Count(&total).Error; err != nil {
		return nil, 0, fmt.Errorf("counting patients by faskes_id: %w", err)
	}

	var rows []PatientWithRisk
	err := db.Raw(`
		WITH latest_risk AS (
			SELECT DISTINCT ON (rs.patient_id)
				rs.patient_id,
				rs.score  AS health_score,
				rs.status AS risk_status,
				rs.top_factors
			FROM risk_scores rs
			INNER JOIN patients p ON rs.patient_id = p.id
			WHERE p.faskes_id = ?
			ORDER BY rs.patient_id, rs.scored_at DESC
		)
		SELECT
			p.id,
			p.faskes_id,
			p.assigned_nakes_id,
			p.username,
			p.password_hash,
			p.full_name,
			p.nik,
			p.phone_number,
			p.companion_name,
			p.companion_phone,
			p.date_of_birth,
			p.sex,
			p.disease_type,
			p.alamat,
			p.enrolled_at,
			p.status,
			p.created_at,
			p.updated_at,
			lr.health_score,
			lr.risk_status,
			lr.top_factors
		FROM patients p
		LEFT JOIN latest_risk lr ON p.id = lr.patient_id
		WHERE p.faskes_id = ?
		ORDER BY p.enrolled_at DESC
		LIMIT ? OFFSET ?
	`, faskesID, faskesID, limit, offset).Scan(&rows).Error
	if err != nil {
		return nil, 0, fmt.Errorf("finding patients with risk by faskes_id: %w", err)
	}
	return rows, total, nil
}

