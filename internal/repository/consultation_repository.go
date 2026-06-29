package repository

import (
	"fmt"
	"sehatiku-backend/internal/entity"
	"sehatiku-backend/internal/model"
	"time"

	"gorm.io/gorm"
)

type ConsultationRepository struct{}

func (r *ConsultationRepository) Create(db *gorm.DB, c *entity.Consultation) error {
	if err := db.Create(c).Error; err != nil {
		return fmt.Errorf("creating consultation: %w", err)
	}
	return nil
}

func (r *ConsultationRepository) FindByPatientID(db *gorm.DB, patientID string) ([]entity.Consultation, error) {
	var rows []entity.Consultation
	if err := db.Where("patient_id = ?", patientID).
		Order("created_at DESC").
		Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("finding consultations for patient %s: %w", patientID, err)
	}
	return rows, nil
}

// FindByNakesID returns consultations for all patients assigned to nakesID within faksesID,
// joined with patients table to resolve patient_name.
func (r *ConsultationRepository) FindByNakesID(db *gorm.DB, nakesID, faksesID string) ([]model.NakesConsultationItem, error) {
	type row struct {
		ID              string
		PatientID       string
		PatientName     string
		ComplaintSince  string
		ComplaintType   string
		ComplaintDetail string
		Status          string
		NakesNote       *string
		RepliedAt       *time.Time
		CreatedAt       time.Time
	}

	var rows []row
	err := db.Raw(`
		SELECT
			c.id,
			c.patient_id,
			p.full_name  AS patient_name,
			c.complaint_since,
			c.complaint_type,
			c.complaint_detail,
			c.status,
			c.nakes_note,
			c.replied_at,
			c.created_at
		FROM consultations c
		JOIN patients p ON p.id = c.patient_id
		WHERE p.assigned_nakes_id = ?
		  AND p.faskes_id         = ?
		ORDER BY c.created_at DESC
	`, nakesID, faksesID).Scan(&rows).Error
	if err != nil {
		return nil, fmt.Errorf("finding consultations for nakes %s: %w", nakesID, err)
	}

	items := make([]model.NakesConsultationItem, len(rows))
	for i, r := range rows {
		items[i] = model.NakesConsultationItem{
			ID:              r.ID,
			PatientID:       r.PatientID,
			PatientName:     r.PatientName,
			ComplaintSince:  r.ComplaintSince,
			ComplaintType:   r.ComplaintType,
			ComplaintDetail: r.ComplaintDetail,
			Status:          r.Status,
			NakesNote:       r.NakesNote,
			RepliedAt:       r.RepliedAt,
			CreatedAt:       r.CreatedAt,
		}
	}
	return items, nil
}

func (r *ConsultationRepository) FindByID(db *gorm.DB, id string) (*entity.Consultation, error) {
	var c entity.Consultation
	if err := db.Where("id = ?", id).First(&c).Error; err != nil {
		return nil, fmt.Errorf("finding consultation %s: %w", id, err)
	}
	return &c, nil
}

// Reply stamps the nakes reply onto a consultation and sets status=replied.
func (r *ConsultationRepository) Reply(db *gorm.DB, id, nakesID, note string, repliedAt time.Time) error {
	updates := map[string]any{
		"nakes_note":          note,
		"replied_by_nakes_id": nakesID,
		"replied_at":          repliedAt,
		"status":              entity.ConsultationStatusReplied,
		"updated_at":          repliedAt,
	}
	result := db.Model(&entity.Consultation{}).
		Where("id = ? AND status = ?", id, entity.ConsultationStatusOpen).
		Updates(updates)
	if result.Error != nil {
		return fmt.Errorf("replying to consultation %s: %w", id, result.Error)
	}
	if result.RowsAffected == 0 {
		return fmt.Errorf("consultation %s already replied", id)
	}
	return nil
}
