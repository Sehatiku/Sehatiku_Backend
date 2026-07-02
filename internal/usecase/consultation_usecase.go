package usecase

import (
	"context"
	"fmt"
	"sehatiku-backend/internal/entity"
	"sehatiku-backend/internal/model"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

type consultationRepo interface {
	Create(db *gorm.DB, c *entity.Consultation) error
	FindByPatientID(db *gorm.DB, patientID string) ([]entity.Consultation, error)
	FindByNakesID(db *gorm.DB, nakesID, faksesID string) ([]model.NakesConsultationItem, error)
	FindByID(db *gorm.DB, id string) (*entity.Consultation, error)
	Reply(db *gorm.DB, id, nakesID, note string, repliedAt time.Time) error
}

type consultationPatientRepo interface {
	FindByID(db *gorm.DB, id string) (*entity.Patient, error)
}

type consultationNakesRepo interface {
	FindByID(db *gorm.DB, id string) (*entity.Nakes, error)
}

// consultationInboxRepo membuat notifikasi inbox in-app pasien (tabel patient_notifications),
// BUKAN baris transport WA/SMS. Pemisahan ini menghilangkan kebingungan lama saat reply
// dokter ditumpangkan ke tabel `notifications` dengan channel=in_app palsu.
type consultationInboxRepo interface {
	Create(db *gorm.DB, n *entity.PatientNotification) error
}

// consultationPushNotifier mengirim push notification native ke pasien saat dokter membalas.
// Optional — nil = skip.
type consultationPushNotifier interface {
	Notify(ctx context.Context, patientID, title, body string, data map[string]string)
}

type ConsultationUseCase struct {
	DB          *gorm.DB
	Repo        consultationRepo
	PatientRepo consultationPatientRepo
	NakesRepo   consultationNakesRepo
	InboxRepo   consultationInboxRepo
	Push        consultationPushNotifier
	Log         *zap.Logger
}

func (u *ConsultationUseCase) CreateConsultation(ctx context.Context, patientID string, req *model.CreateConsultationRequest) (*model.ConsultationResponse, error) {
	c := &entity.Consultation{
		PatientID:       patientID,
		ComplaintSince:  req.ComplaintSince,
		ComplaintType:   req.ComplaintType,
		ComplaintDetail: req.ComplaintDetail,
		Status:          entity.ConsultationStatusOpen,
	}
	if err := u.Repo.Create(u.DB, c); err != nil {
		return nil, fmt.Errorf("creating consultation for patient %s: %w", patientID, err)
	}

	u.Log.Info("consultation created",
		zap.String("patient_id", patientID),
		zap.String("consultation_id", c.ID),
	)

	return toConsultationResponse(c), nil
}

func (u *ConsultationUseCase) GetPatientConsultations(ctx context.Context, patientID string) ([]model.ConsultationResponse, error) {
	rows, err := u.Repo.FindByPatientID(u.DB, patientID)
	if err != nil {
		return nil, fmt.Errorf("fetching consultations for patient %s: %w", patientID, err)
	}

	out := make([]model.ConsultationResponse, len(rows))
	for i, r := range rows {
		rc := r
		out[i] = *toConsultationResponse(&rc)
	}
	return out, nil
}

func (u *ConsultationUseCase) GetNakesConsultations(ctx context.Context, nakesID, faksesID string) ([]model.NakesConsultationItem, error) {
	items, err := u.Repo.FindByNakesID(u.DB, nakesID, faksesID)
	if err != nil {
		return nil, fmt.Errorf("fetching consultations for nakes %s: %w", nakesID, err)
	}
	return items, nil
}

func (u *ConsultationUseCase) ReplyConsultation(ctx context.Context, consultationID, nakesID string, req *model.ReplyConsultationRequest) error {
	c, err := u.Repo.FindByID(u.DB, consultationID)
	if err != nil {
		return fmt.Errorf("finding consultation %s: %w", consultationID, err)
	}

	if c.Status == entity.ConsultationStatusReplied {
		return fmt.Errorf("consultation %s already replied", consultationID)
	}

	now := time.Now()
	if err := u.Repo.Reply(u.DB, consultationID, nakesID, req.NakesNote, now); err != nil {
		return fmt.Errorf("replying to consultation %s: %w", consultationID, err)
	}

	u.Log.Info("consultation replied",
		zap.String("consultation_id", consultationID),
		zap.String("nakes_id", nakesID),
	)

	// Resolve nakes name for the inbox payload. Fall back to nakes_id if lookup fails.
	nakesName := nakesID
	if nakes, err := u.NakesRepo.FindByID(u.DB, nakesID); err == nil {
		nakesName = nakes.FullName
	}

	// Tulis ke inbox in-app pasien (patient_notifications). Non-kritis: gagal hanya di-log,
	// reply tetap dianggap sukses (fire-and-forget, konsisten dgn perilaku lama).
	payload := fmt.Sprintf(`{"nakes_name":%q}`, nakesName)
	consultationIDRef := consultationID
	notif := &entity.PatientNotification{
		PatientID:      c.PatientID,
		Type:           entity.PatientNotifTypeConsultationReply,
		Title:          "Balasan dari dokter",
		Body:           req.NakesNote,
		Payload:        &payload,
		ConsultationID: &consultationIDRef,
	}
	if dbErr := u.InboxRepo.Create(u.DB, notif); dbErr != nil {
		u.Log.Warn("failed to create in-app notification for consultation reply",
			zap.String("consultation_id", consultationID),
			zap.String("patient_id", c.PatientID),
			zap.Error(dbErr),
		)
	}

	// Push notification best-effort, fire-and-forget — never block the reply response
	// (same pattern as ScoringUseCase's acute escalation fan-out in scoring_usecase.go).
	if u.Push != nil {
		go u.Push.Notify(context.Background(), c.PatientID, "Balasan dari dokter", req.NakesNote,
			map[string]string{"type": "consultation_reply", "consultation_id": consultationID})
	}

	return nil
}

func toConsultationResponse(c *entity.Consultation) *model.ConsultationResponse {
	return &model.ConsultationResponse{
		ID:              c.ID,
		PatientID:       c.PatientID,
		ComplaintSince:  c.ComplaintSince,
		ComplaintType:   c.ComplaintType,
		ComplaintDetail: c.ComplaintDetail,
		Status:          c.Status,
		NakesNote:       c.NakesNote,
		RepliedAt:       c.RepliedAt,
		CreatedAt:       c.CreatedAt,
	}
}
