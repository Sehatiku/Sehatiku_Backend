package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"mime/multipart"
	"sehatiku-backend/internal/entity"
	"sehatiku-backend/internal/gateway/ocr"
	"sehatiku-backend/internal/gateway/whatsapp"
	"sehatiku-backend/internal/model"
	"time"

	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// waSendTimeout membatasi berapa lama registrasi menunggu satu pengiriman kredensial WA
// sebelum dianggap gagal. Cukup pendek supaya response registrasi tidak menggantung saat
// WhatsApp putus, tapi cukup longgar untuk pengiriman normal saat koneksi sehat.
const waSendTimeout = 15 * time.Second

// ErrAssignedNakesInvalid dikembalikan bila assigned_nakes_id pada request tidak
// merujuk ke nakes mana pun, atau merujuk ke nakes milik faskes lain (isolasi tenant).
var ErrAssignedNakesInvalid = errors.New("assigned_nakes_id tidak valid atau bukan milik faskes ini")

type PatientRegistrationUseCase struct {
	DB               *gorm.DB
	PatientRepo      patientRepo
	NakesRepo        patientRegNakesRepo
	NotificationRepo notificationRepo
	OCRGateway       *ocr.KTPOCRGateway
	WhatsApp         *whatsapp.WhatsAppGateway
	Log              *zap.Logger
}

type patientRepo interface {
	FindByNIK(db *gorm.DB, nik string) (*entity.Patient, error)
	FindByUsername(db *gorm.DB, username string) (*entity.Patient, error)
	Create(db *gorm.DB, entity *entity.Patient) error
}

type patientRegNakesRepo interface {
	FindByID(db *gorm.DB, id string) (*entity.Nakes, error)
}

type notificationRepo interface {
	Create(db *gorm.DB, entity *entity.Notification) error
	Update(db *gorm.DB, entity *entity.Notification) error
}

func (u *PatientRegistrationUseCase) ScanKTP(ctx context.Context, file multipart.File, filename string) (*model.KTPOCRResponse, error) {
	result, err := u.OCRGateway.ExtractKTP(ctx, file, filename)
	if err != nil {
		return nil, err
	}
	return convertOCRResult(result), nil
}

func (u *PatientRegistrationUseCase) RegisterPatient(ctx context.Context, faskesID string, req *model.PatientRegisterRequest) (*model.PatientRegisterResponse, error) {
	// Validasi assigned_nakes_id dikirim faskes: harus nakes yang benar-benar ada DAN
	// milik faskes yang sedang login. Pesan diseragamkan agar keberadaan nakes milik
	// faskes lain tidak bocor lintas tenant.
	nakes, err := u.NakesRepo.FindByID(u.DB, req.AssignedNakesID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrAssignedNakesInvalid
		}
		return nil, fmt.Errorf("checking assigned nakes: %w", err)
	}
	if nakes.FaskesID != faskesID {
		return nil, ErrAssignedNakesInvalid
	}

	_, err = u.PatientRepo.FindByNIK(u.DB, req.NIK)
	if err == nil {
		return nil, ErrNIKAlreadyExists
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("checking NIK availability: %w", err)
	}

	_, err = u.PatientRepo.FindByUsername(u.DB, req.Username)
	if err == nil {
		return nil, ErrUsernameAlreadyExists
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("checking username availability: %w", err)
	}

	dob, err := time.Parse("2006-01-02", req.DateOfBirth)
	if err != nil {
		return nil, fmt.Errorf("format date_of_birth tidak valid (gunakan YYYY-MM-DD): %w", err)
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("hashing password: %w", err)
	}

	now := time.Now()
	patient := &entity.Patient{
		FaskesID:        faskesID,
		AssignedNakesID: req.AssignedNakesID,
		Username:        req.Username,
		PasswordHash:    string(hash),
		FullName:        req.FullName,
		NIK:             req.NIK,
		Alamat:          req.Alamat,
		PhoneNumber:     req.PhoneNumber,
		CompanionName:   req.CompanionName,
		CompanionPhone:  req.CompanionPhone,
		DateOfBirth:     &dob,
		Sex:             req.Sex,
		DiseaseType:     req.DiseaseType,
		Status:          "active",
		EnrolledAt:      now,
	}

	if err := u.PatientRepo.Create(u.DB, patient); err != nil {
		return nil, fmt.Errorf("creating patient: %w", err)
	}

	u.Log.Info("patient registered", zap.String("patient_id", patient.ID), zap.String("faskes_id", faskesID))

	// Kirim kredensial login ke WA pasien & pendamping secara SINKRON (timeout terbatas)
	// lalu catat hasilnya ke tabel notifications untuk audit. Kegagalan WA TIDAK boleh
	// menggagalkan registrasi yang sudah tersimpan — hasilnya hanya tercermin di
	// wa_delivery pada response, dan faskes tetap menerima kredensial sebagai cadangan.
	var wa model.WADeliveryStatus

	wa.Patient = u.sendCredentialAndRecord(
		ctx, patient.ID, patient.PhoneNumber, "patient", patient.FullName, patient.Username,
		func(c context.Context) error {
			return u.WhatsApp.SendRegistrationCredentials(c, patient.PhoneNumber, patient.FullName, patient.Username, req.Password)
		},
	)

	if patient.CompanionPhone != "" {
		wa.Companion = u.sendCredentialAndRecord(
			ctx, patient.ID, patient.CompanionPhone, "companion", patient.CompanionName, patient.Username,
			func(c context.Context) error {
				return u.WhatsApp.SendCompanionRegistrationCredentials(c, patient.CompanionPhone, patient.CompanionName, patient.FullName, patient.Username, req.Password)
			},
		)
	}

	return &model.PatientRegisterResponse{
		PatientID:   patient.ID,
		FaskesID:    patient.FaskesID,
		FullName:    patient.FullName,
		NIK:         patient.NIK,
		DiseaseType: patient.DiseaseType,
		EnrolledAt:  patient.EnrolledAt,
		Credentials: model.PatientCredentials{
			Username: patient.Username,
			Password: req.Password,
		},
		WADelivery: wa,
	}, nil
}

// sendCredentialAndRecord mengirim satu pesan kredensial via WA (dibungkus sendFn) dan
// mencatat percobaannya ke tabel notifications sebagai audit. Mengembalikan "sent" bila
// WA sukses, "failed" bila gagal. Sengaja tidak mengembalikan error: kegagalan WA atau DB
// tidak boleh menggagalkan registrasi yang sudah tersimpan — cukup tercermin di status & log.
func (u *PatientRegistrationUseCase) sendCredentialAndRecord(
	ctx context.Context,
	patientID, recipientPhone, recipientRole, recipientName, username string,
	sendFn func(context.Context) error,
) string {
	// Payload audit sengaja TANPA password — hanya metadata non-sensitif.
	// Disimpan sebagai string JSON: kolom `payload` bertipe jsonb, dan driver pgx
	// mengirim []byte sebagai bytea (gagal di-parse jsonb) sedangkan string dikirim
	// sebagai teks yang benar di-parse Postgres menjadi jsonb.
	payload, _ := json.Marshal(map[string]string{
		"username":       username,
		"recipient_name": recipientName,
	})

	pid := patientID
	notif := &entity.Notification{
		PatientID:      &pid,
		RecipientPhone: recipientPhone,
		RecipientRole:  recipientRole,
		MessageType:    "credential_blast",
		Channel:        "whatsapp",
		Payload:        string(payload),
		Status:         "queued",
		QueuedAt:       time.Now(),
	}
	recorded := true
	if err := u.NotificationRepo.Create(u.DB, notif); err != nil {
		// Gagal membuat baris audit bukan alasan membatalkan pengiriman kredensial.
		u.Log.Warn("failed to record credential notification",
			zap.String("patient_id", patientID), zap.String("recipient_role", recipientRole), zap.Error(err))
		recorded = false
	}

	sendCtx, cancel := context.WithTimeout(ctx, waSendTimeout)
	defer cancel()
	sendErr := sendFn(sendCtx)

	status := "sent"
	if sendErr != nil {
		status = "failed"
		u.Log.Warn("failed to send wa registration credentials",
			zap.String("patient_id", patientID), zap.String("recipient_role", recipientRole), zap.Error(sendErr))
	}

	if recorded {
		notif.Status = status
		if sendErr != nil {
			reason := sendErr.Error()
			notif.ErrorReason = &reason
		} else {
			sentAt := time.Now()
			notif.SentAt = &sentAt
		}
		if err := u.NotificationRepo.Update(u.DB, notif); err != nil {
			u.Log.Warn("failed to update credential notification status",
				zap.String("patient_id", patientID), zap.String("notification_id", notif.ID), zap.Error(err))
		}
	}

	return status
}
