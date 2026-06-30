package usecase

import (
	"context"
	"errors"
	"fmt"
	"sehatiku-backend/internal/entity"
	"sehatiku-backend/internal/model"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

var (
	ErrEscalationNotFound = errors.New("escalation not found")
	ErrEscalationClosed   = errors.New("escalation already closed")
)

type escalationRepository interface {
	Create(db *gorm.DB, e *entity.Escalation) error
	FindActiveByPatientTier(db *gorm.DB, patientID, tier string) (*entity.Escalation, error)
	FindByID(db *gorm.DB, id string) (*entity.Escalation, error)
	UpdateStatus(db *gorm.DB, id, status string, at time.Time) error
	FindByFaskes(db *gorm.DB, faskesID, status, tier string, limit, offset int) ([]model.EscalationQueueItem, int64, error)
}

type escalationRiskReader interface {
	FindLatestStatus(db *gorm.DB, patientID, excludeID string) (string, bool, error)
}

type EscalationUseCase struct {
	DB       *gorm.DB
	Repo     escalationRepository
	RiskRepo escalationRiskReader
	Log      *zap.Logger
}

// EvaluateAcute membuat eskalasi acute_today ketika status pasien BARU bertransisi ke
// 'bahaya'. Best-effort: error dikembalikan agar caller bisa me-log; tidak pernah panic.
// Dipanggil fire-and-forget dari ScoringUseCase setelah risk score tersimpan.
func (u *EscalationUseCase) EvaluateAcute(ctx context.Context, patient *entity.Patient, score *entity.RiskScore) error {
	if score.Status != entity.RiskStatusBahaya {
		return nil
	}

	prevStatus, found, err := u.RiskRepo.FindLatestStatus(u.DB, patient.ID, score.ID)
	if err != nil {
		return fmt.Errorf("checking previous status for patient %s: %w", patient.ID, err)
	}
	// Hanya eskalasi pada transisi BARU ke bahaya. Kalau sebelumnya sudah bahaya, ini kasus
	// menetap (bukan kejadian akut baru) — lewati supaya tidak banjir tiap hari.
	if found && prevStatus == entity.RiskStatusBahaya {
		return nil
	}

	active, err := u.Repo.FindActiveByPatientTier(u.DB, patient.ID, entity.EscalationTierAcuteToday)
	if err != nil {
		return fmt.Errorf("checking active escalation for patient %s: %w", patient.ID, err)
	}
	if active != nil {
		u.Log.Info("acute escalation skipped: one already active",
			zap.String("patient_id", patient.ID),
			zap.String("escalation_id", active.ID))
		return nil
	}

	esc := &entity.Escalation{
		PatientID:       patient.ID,
		RiskScoreID:     score.ID,
		FaskesID:        patient.FaskesID,
		AssignedNakesID: patient.AssignedNakesID,
		Tier:            entity.EscalationTierAcuteToday,
		Channel:         entity.NotificationChannelWhatsApp,
		Status:          entity.EscalationStatusSent,
		SentAt:          time.Now(),
	}
	if err := u.Repo.Create(u.DB, esc); err != nil {
		return fmt.Errorf("creating acute escalation for patient %s: %w", patient.ID, err)
	}

	u.Log.Info("acute escalation created",
		zap.String("patient_id", patient.ID),
		zap.String("escalation_id", esc.ID),
		zap.String("assigned_nakes_id", patient.AssignedNakesID))
	return nil
}

// GetQueue mengembalikan satu halaman antrean eskalasi untuk faskes (acute lebih dulu).
func (u *EscalationUseCase) GetQueue(ctx context.Context, faskesID, status, tier string, page, size int) ([]model.EscalationQueueItem, model.PageMetadata, error) {
	if page < 1 {
		page = 1
	}
	if size < 1 || size > 100 {
		size = 20
	}
	offset := (page - 1) * size

	items, total, err := u.Repo.FindByFaskes(u.DB, faskesID, status, tier, size, offset)
	if err != nil {
		return nil, model.PageMetadata{}, fmt.Errorf("listing escalations for faskes %s: %w", faskesID, err)
	}
	if items == nil {
		items = []model.EscalationQueueItem{}
	}
	totalPage := (total + int64(size) - 1) / int64(size)
	paging := model.PageMetadata{Page: page, Size: size, TotalItem: total, TotalPage: totalPage}
	return items, paging, nil
}

// View menandai eskalasi 'viewed' (idempoten: jika sudah viewed/acted/dismissed, no-op).
func (u *EscalationUseCase) View(ctx context.Context, id, faskesID string) error {
	esc, err := u.loadOwned(id, faskesID)
	if err != nil {
		return err
	}
	if esc.Status != entity.EscalationStatusSent {
		return nil
	}
	if err := u.Repo.UpdateStatus(u.DB, id, entity.EscalationStatusViewed, time.Now()); err != nil {
		return fmt.Errorf("marking escalation %s viewed: %w", id, err)
	}
	return nil
}

// Act menandai eskalasi sudah ditindaklanjuti.
func (u *EscalationUseCase) Act(ctx context.Context, id, faskesID string) error {
	return u.closeEscalation(id, faskesID, entity.EscalationStatusActed)
}

// Dismiss menandai eskalasi diabaikan.
func (u *EscalationUseCase) Dismiss(ctx context.Context, id, faskesID string) error {
	return u.closeEscalation(id, faskesID, entity.EscalationStatusDismissed)
}

func (u *EscalationUseCase) closeEscalation(id, faskesID, newStatus string) error {
	esc, err := u.loadOwned(id, faskesID)
	if err != nil {
		return err
	}
	if esc.Status == entity.EscalationStatusActed || esc.Status == entity.EscalationStatusDismissed {
		return ErrEscalationClosed
	}
	if err := u.Repo.UpdateStatus(u.DB, id, newStatus, time.Now()); err != nil {
		return fmt.Errorf("closing escalation %s: %w", id, err)
	}
	return nil
}

// loadOwned memuat eskalasi dan memastikan milik faskes pemanggil. Tidak ditemukan ATAU
// milik faskes lain → ErrEscalationNotFound (404, bukan 403 — tidak bocorkan keberadaan
// lintas tenant).
func (u *EscalationUseCase) loadOwned(id, faskesID string) (*entity.Escalation, error) {
	esc, err := u.Repo.FindByID(u.DB, id)
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrEscalationNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("loading escalation %s: %w", id, err)
	}
	if esc.FaskesID != faskesID {
		return nil, ErrEscalationNotFound
	}
	return esc, nil
}
