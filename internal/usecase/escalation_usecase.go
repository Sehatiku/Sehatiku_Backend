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
	ErrInvalidFeedback    = errors.New("invalid feedback value")
)

type escalationRepository interface {
	Create(db *gorm.DB, e *entity.Escalation) error
	ExistsActiveOrRecent(db *gorm.DB, patientID, tier string, since time.Time) (bool, error)
	FindByID(db *gorm.DB, id string) (*entity.Escalation, error)
	UpdateStatus(db *gorm.DB, id, status string, at time.Time) error
	FindByFaskes(db *gorm.DB, faskesID, status, tier string, limit, offset int) ([]model.EscalationQueueItem, int64, error)
	SetFeedback(db *gorm.DB, id, feedback, nakesID string, at time.Time) error
	CountTodayByNakes(db *gorm.DB, nakesID string, now time.Time) (int64, error)
}

type escalationRiskReader interface {
	FindLatestStatus(db *gorm.DB, patientID, excludeID string) (string, bool, error)
}

// escalationHealthLogReader mendeteksi pembacaan ekstrem hari ini (pemicu acute selain
// transisi status). Optional — nil = trigger ekstrem dilewati.
type escalationHealthLogReader interface {
	HasExtremeReadingToday(db *gorm.DB, patientID string, glucoseHigh, glucoseLow, systolicHigh, diastolicHigh float64) (bool, error)
}

type escalationNotifier interface {
	SendEscalationToNakes(ctx context.Context, toPhone, nakesName, patientName, riskStatus string) error
	SendEscalationToPatient(ctx context.Context, toPhone, patientName string) error
	SendEscalationToCompanion(ctx context.Context, toPhone, companionName, patientName string) error
}

// escalationPushNotifier mengirim push notification native ke pasien. Optional — nil = skip
// (server tetap jalan tanpa fitur push, sama falsafah dengan WA/InboxRepo).
type escalationPushNotifier interface {
	Notify(ctx context.Context, patientID, title, body string, data map[string]string)
}

type escalationNotifRepo interface {
	Create(db *gorm.DB, n *entity.Notification) error
	MarkStatus(db *gorm.DB, id, status string, errReason *string) error
}

type escalationInboxRepo interface {
	Create(db *gorm.DB, n *entity.PatientNotification) error
}

type escalationNakesRepo interface {
	FindByID(db *gorm.DB, id string) (*entity.Nakes, error)
}

type EscalationUseCase struct {
	DB       *gorm.DB
	Repo     escalationRepository
	RiskRepo escalationRiskReader
	Log      *zap.Logger

	// Fan-out dependencies (optional; nil = skip that channel). Wired in Increment 2.
	NakesRepo   escalationNakesRepo
	WA          escalationNotifier
	NotifRepo   escalationNotifRepo
	InboxRepo   escalationInboxRepo
	Push        escalationPushNotifier
	AlertBudget int // 0 = unlimited

	// Acute trigger tuning (Increment final). HealthLogRepo optional (nil = no extreme check).
	HealthLogRepo      escalationHealthLogReader
	AcuteCooldown      time.Duration // 0 = hanya dedup alert terbuka, tanpa cooldown waktu
	AcuteGlucoseHigh   float64
	AcuteGlucoseLow    float64
	AcuteSystolicHigh  float64
	AcuteDiastolicHigh float64
}

// EvaluateAcute membuat eskalasi acute_today ketika status pasien BARU bertransisi ke
// 'bahaya'. Best-effort: error dikembalikan agar caller bisa me-log; tidak pernah panic.
// Dipanggil fire-and-forget dari ScoringUseCase setelah risk score tersimpan.
func (u *EscalationUseCase) EvaluateAcute(ctx context.Context, patient *entity.Patient, score *entity.RiskScore) error {
	// Pemicu 1: transisi BARU ke bahaya (skor sebelumnya bukan bahaya). Kalau sudah bahaya
	// sebelumnya, itu kasus menetap (bukan kejadian akut baru).
	transition := false
	if score.Status == entity.RiskStatusBahaya {
		prevStatus, found, err := u.RiskRepo.FindLatestStatus(u.DB, patient.ID, score.ID)
		if err != nil {
			return fmt.Errorf("checking previous status for patient %s: %w", patient.ID, err)
		}
		transition = !found || prevStatus != entity.RiskStatusBahaya
	}

	// Pemicu 2: pembacaan ekstrem hari ini (jaring pengaman bila ML tak menandai bahaya).
	extreme := false
	if u.HealthLogRepo != nil {
		hit, err := u.HealthLogRepo.HasExtremeReadingToday(u.DB, patient.ID,
			u.AcuteGlucoseHigh, u.AcuteGlucoseLow, u.AcuteSystolicHigh, u.AcuteDiastolicHigh)
		if err != nil {
			u.Log.Warn("extreme reading check failed", zap.String("patient_id", patient.ID), zap.Error(err))
		} else {
			extreme = hit
		}
	}

	if !transition && !extreme {
		return nil
	}

	// Dedup + cooldown: lewati bila ada alert acute terbuka atau yang baru dibuat (cooldown).
	since := time.Now().Add(-u.AcuteCooldown)
	blocked, err := u.Repo.ExistsActiveOrRecent(u.DB, patient.ID, entity.EscalationTierAcuteToday, since)
	if err != nil {
		return fmt.Errorf("checking active/recent escalation for patient %s: %w", patient.ID, err)
	}
	if blocked {
		u.Log.Info("acute escalation skipped: active or within cooldown",
			zap.String("patient_id", patient.ID))
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

	u.fanOutAcute(ctx, esc, patient, score.Status)
	return nil
}

// fanOutAcute mengirim notifikasi keluar untuk eskalasi acute: inbox in-app pasien (selalu),
// lalu WA ke nakes/pasien/pendamping (di-gate alert budget). Best-effort: tiap kegagalan
// hanya di-log dan dicatat sebagai notifications.status=failed.
func (u *EscalationUseCase) fanOutAcute(ctx context.Context, esc *entity.Escalation, patient *entity.Patient, riskStatus string) {
	// 1. Inbox in-app pasien — bukan WA, tidak kena budget.
	if u.InboxRepo != nil {
		inbox := &entity.PatientNotification{
			PatientID: patient.ID,
			Type:      entity.PatientNotifTypeEscalation,
			Title:     "Kondisi Anda perlu perhatian",
			Body:      "Tim kesehatan Anda telah diberi tahu. Mohon segera hubungi faskes Anda.",
		}
		if err := u.InboxRepo.Create(u.DB, inbox); err != nil {
			u.Log.Warn("escalation inbox create failed",
				zap.String("patient_id", patient.ID), zap.Error(err))
		}
	}

	// 1b. Push notification native — sama teks dengan inbox, di luar alert budget WA.
	if u.Push != nil {
		u.Push.Notify(ctx, patient.ID,
			"Kondisi Anda perlu perhatian",
			"Tim kesehatan Anda telah diberi tahu. Mohon segera hubungi faskes Anda.",
			map[string]string{"type": "escalation", "escalation_id": esc.ID})
	}

	// 2. WA blast — butuh gateway + audit repo, dan tidak melewati alert budget.
	if u.WA == nil || u.NotifRepo == nil {
		return
	}
	if u.overBudget(esc.AssignedNakesID) {
		u.Log.Info("escalation WA blast skipped: alert budget exceeded",
			zap.String("assigned_nakes_id", esc.AssignedNakesID),
			zap.String("escalation_id", esc.ID))
		return
	}

	// 2a. Nakes WA (butuh nomor nakes).
	if u.NakesRepo != nil {
		if nakes, err := u.NakesRepo.FindByID(u.DB, patient.AssignedNakesID); err == nil && nakes.PhoneNumber != "" {
			nakesID := nakes.ID
			u.sendAndAudit(esc, entity.RecipientRoleNakes, nakes.PhoneNumber, nil, &nakesID, func() error {
				return u.WA.SendEscalationToNakes(ctx, nakes.PhoneNumber, nakes.FullName, patient.FullName, riskStatus)
			})
		} else if err != nil {
			u.Log.Warn("escalation: could not load nakes for WA",
				zap.String("nakes_id", patient.AssignedNakesID), zap.Error(err))
		}
	}

	// 2b. Patient WA.
	if patient.PhoneNumber != "" {
		patientID := patient.ID
		u.sendAndAudit(esc, entity.RecipientRolePatient, patient.PhoneNumber, &patientID, nil, func() error {
			return u.WA.SendEscalationToPatient(ctx, patient.PhoneNumber, patient.FullName)
		})
	}

	// 2c. Companion WA.
	if patient.CompanionPhone != "" {
		patientID := patient.ID
		u.sendAndAudit(esc, entity.RecipientRoleCompanion, patient.CompanionPhone, &patientID, nil, func() error {
			return u.WA.SendEscalationToCompanion(ctx, patient.CompanionPhone, patient.CompanionName, patient.FullName)
		})
	}
}

// overBudget mengembalikan true bila nakes sudah melampaui kuota eskalasi harian (WA di-skip,
// eskalasi & inbox tetap dibuat). AlertBudget<=0 berarti tanpa batas. Error hitung → tidak
// menahan (fail-open) supaya kegagalan budget tidak diam-diam memblok notifikasi penting.
func (u *EscalationUseCase) overBudget(nakesID string) bool {
	if u.AlertBudget <= 0 {
		return false
	}
	count, err := u.Repo.CountTodayByNakes(u.DB, nakesID, time.Now())
	if err != nil {
		u.Log.Warn("alert budget check failed; allowing send", zap.String("nakes_id", nakesID), zap.Error(err))
		return false
	}
	return count > int64(u.AlertBudget)
}

// sendAndAudit mencatat satu baris notifications (queued), memanggil send, lalu menandai
// sent/failed. recipient_phone wajib; patientID/nakesID memenuhi CHECK chk_recipient_target.
func (u *EscalationUseCase) sendAndAudit(esc *entity.Escalation, role, phone string, patientID, nakesID *string, send func() error) {
	escID := esc.ID
	notif := &entity.Notification{
		PatientID:      patientID,
		NakesID:        nakesID,
		EscalationID:   &escID,
		RecipientPhone: phone,
		RecipientRole:  role,
		MessageType:    entity.MessageTypeEscalation,
		Channel:        entity.NotificationChannelWhatsApp,
		Status:         entity.NotificationStatusQueued,
		QueuedAt:       time.Now(),
	}
	if err := u.NotifRepo.Create(u.DB, notif); err != nil {
		u.Log.Warn("escalation notification audit create failed",
			zap.String("role", role), zap.Error(err))
		return
	}
	if err := send(); err != nil {
		es := err.Error()
		_ = u.NotifRepo.MarkStatus(u.DB, notif.ID, entity.NotificationStatusFailed, &es)
		u.Log.Warn("escalation WA send failed",
			zap.String("role", role), zap.String("escalation_id", esc.ID), zap.Error(err))
		return
	}
	_ = u.NotifRepo.MarkStatus(u.DB, notif.ID, entity.NotificationStatusSent, nil)
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

// SetFeedback menyimpan label nakes (accurate/inaccurate) pada eskalasi — label emas untuk
// training model. Boleh diisi tanpa memandang status lifecycle.
func (u *EscalationUseCase) SetFeedback(ctx context.Context, id, faskesID, nakesID, feedback string) error {
	if feedback != entity.EscalationFeedbackAccurate && feedback != entity.EscalationFeedbackInaccurate {
		return ErrInvalidFeedback
	}
	if _, err := u.loadOwned(id, faskesID); err != nil {
		return err
	}
	if err := u.Repo.SetFeedback(u.DB, id, feedback, nakesID, time.Now()); err != nil {
		return fmt.Errorf("setting feedback on escalation %s: %w", id, err)
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
