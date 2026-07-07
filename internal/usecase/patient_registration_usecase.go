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
	"sehatiku-backend/internal/helper"
	"sehatiku-backend/internal/model"
	"sehatiku-backend/internal/repository"
	"time"

	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// waWarmupPrefillText adalah teks yang sudah terisi di chat penerima saat membuka link
// wa.me. Penerima cukup mengirimnya untuk "menghangatkan" kontak; isi pesan tidak dipakai
// untuk pencocokan (warm-up dicocokkan berdasarkan nomor pengirim).
const waWarmupPrefillText = "HALO SEHATIKU, saya ingin menerima detail akun saya."

// warmupStatusPending / warmupStatusUnavailable adalah nilai field wa_warmup.status.
const (
	warmupStatusPending     = "pending"
	warmupStatusUnavailable = "unavailable"
)

// ErrAssignedNakesInvalid dikembalikan bila assigned_nakes_id pada request tidak
// merujuk ke nakes mana pun, atau merujuk ke nakes milik faskes lain (isolasi tenant).
var ErrAssignedNakesInvalid = errors.New("assigned_nakes_id tidak valid atau bukan milik faskes ini")

type PatientRegistrationUseCase struct {
	DB                *gorm.DB
	PatientRepo       patientRepo
	NakesRepo         patientRegNakesRepo
	NotificationRepo  notificationRepo
	PendingCredential pendingCredentialStasher
	BaselineRepo      baselineRepo
	OCRGateway        *ocr.KTPOCRGateway
	WhatsApp          *whatsapp.WhatsAppGateway
	Log               *zap.Logger
}

// pendingCredentialStasher menyimpan kredensial yang menunggu warm-up (di Redis).
type pendingCredentialStasher interface {
	Stash(ctx context.Context, phone string, data repository.PendingCredential, ttl time.Duration) error
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
}

type baselineRepo interface {
	Create(db *gorm.DB, baseline *entity.PatientClinicalBaseline) error
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
		// Normalisasi ke format internasional telanjang (62...) agar cocok dengan nomor
		// pengirim WA (jid.User) saat pasien/pendamping mencatat log via WhatsApp, dan
		// saat warm-up credential dikirim. Tanpa ini, "0812.."/"+62 812.." di DB tak
		// pernah match "62812.." dari WA. Lihat helper.NormalizePhoneID.
		PhoneNumber:     helper.NormalizePhoneID(req.PhoneNumber),
		CompanionName:   req.CompanionName,
		CompanionPhone:  helper.NormalizePhoneID(req.CompanionPhone),
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

	// Create clinical baseline. Non-fatal: patient is already persisted; a baseline
	// failure does not roll back registration. Faskes can re-submit baseline separately
	// if needed.
	baseline := buildBaseline(patient.ID, req.Baseline)
	if err := u.BaselineRepo.Create(u.DB, baseline); err != nil {
		u.Log.Warn("failed to create clinical baseline",
			zap.String("patient_id", patient.ID), zap.Error(err))
	}

	// Alur warm-up: backend TIDAK mengirim kredensial duluan (WhatsApp memblokir kontak
	// baru dengan error 463). Sebagai gantinya, catat baris audit `queued`, simpan kredensial
	// menunggu di Redis, dan kembalikan link wa.me supaya pasien/pendamping menghubungi bot
	// lebih dulu — saat mereka masuk, WAInboundUseCase mengirim kredensial. Faskes selalu
	// punya kredensial dari response sebagai cadangan terjamin.
	botPhone := u.WhatsApp.BotPhone()
	warmup := model.WAWarmupStatus{
		BotPhone: botPhone,
		Status:   warmupStatusPending,
	}
	if botPhone == "" {
		warmup.Status = warmupStatusUnavailable
	}

	warmup.PatientLink = u.prepareWarmup(ctx, patient.ID, patient.PhoneNumber,
		entity.RecipientRolePatient, patient.FullName, "", patient.Username, req.Password, botPhone)
	warmup.PatientDirectLink = helper.BuildDirectInviteLink(patient.PhoneNumber,
		entity.RecipientRolePatient, patient.FullName, "", patient.Username, warmup.PatientLink)

	if patient.CompanionPhone != "" {
		warmup.CompanionLink = u.prepareWarmup(ctx, patient.ID, patient.CompanionPhone,
			entity.RecipientRoleCompanion, patient.CompanionName, patient.FullName, patient.Username, req.Password, botPhone)
		warmup.CompanionDirectLink = helper.BuildDirectInviteLink(patient.CompanionPhone,
			entity.RecipientRoleCompanion, patient.CompanionName, patient.FullName, patient.Username, warmup.CompanionLink)
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
		WAWarmup: warmup,
	}, nil
}

// prepareWarmup mencatat baris audit notifications (status queued, TANPA password),
// menyimpan kredensial menunggu warm-up di Redis (di-keyed nomor penerima), dan
// mengembalikan link wa.me first-contact. Kegagalan DB/Redis di-log tapi TIDAK
// menggagalkan registrasi yang sudah tersimpan (partial failure, be_implementation §6) —
// faskes tetap memegang kredensial di response.
func (u *PatientRegistrationUseCase) prepareWarmup(
	ctx context.Context,
	patientID, recipientPhone, recipientRole, recipientName, patientName, username, password, botPhone string,
) string {
	// Payload audit sengaja TANPA password — hanya metadata non-sensitif. Disimpan sebagai
	// string JSON karena kolom `payload` bertipe jsonb (driver pgx mengirim []byte sebagai
	// bytea yang gagal di-parse jsonb; string dikirim sebagai teks yang benar).
	payload, _ := json.Marshal(map[string]string{
		"username":       username,
		"recipient_name": recipientName,
	})

	pid := patientID
	notif := &entity.Notification{
		PatientID:      &pid,
		RecipientPhone: recipientPhone,
		RecipientRole:  recipientRole,
		MessageType:    entity.MessageTypeCredentialBlast,
		Channel:        "whatsapp",
		Payload:        string(payload),
		Status:         entity.NotificationStatusQueued,
		QueuedAt:       time.Now(),
	}
	if err := u.NotificationRepo.Create(u.DB, notif); err != nil {
		u.Log.Warn("failed to record credential notification",
			zap.String("patient_id", patientID), zap.String("recipient_role", recipientRole), zap.Error(err))
	}

	if err := u.PendingCredential.Stash(ctx, recipientPhone, repository.PendingCredential{
		Role:           recipientRole,
		RecipientName:  recipientName,
		PatientName:    patientName,
		Username:       username,
		Password:       password,
		NotificationID: notif.ID,
	}, repository.PendingCredentialDefaultTTL); err != nil {
		u.Log.Warn("failed to stash pending credential for warm-up",
			zap.String("patient_id", patientID), zap.String("recipient_role", recipientRole),
			zap.String("phone", helper.MaskPhone(recipientPhone)), zap.Error(err))
	}

	return helper.BuildWAMeLink(botPhone, waWarmupPrefillText)
}

func buildBaseline(patientID string, b model.PatientBaselineRequest) *entity.PatientClinicalBaseline {
	return &entity.PatientClinicalBaseline{
		PatientID:             patientID,
		RecordedAt:            time.Now(),
		AgeYears:              b.AgeYears,
		Sex:                   b.Sex,
		BMI:                   b.BMI,
		BMICategory:           b.BMICategory,
		WaistCircumferenceCm:  b.WaistCircumferenceCm,
		CentralObesity:        derefBool(b.CentralObesity),
		SmokingStatus:         b.SmokingStatus,
		AlcoholUse:            derefBool(b.AlcoholUse),
		PhysicalActivity:      b.PhysicalActivity,
		FamilyHistoryDiabetes: derefBool(b.FamilyHistoryDiabetes),
		FamilyHistoryCVD:      derefBool(b.FamilyHistoryCVD),
		SystolicBPMmhg:        b.SystolicBPMmhg,
		DiastolicBPMmhg:       b.DiastolicBPMmhg,
		HypertensionStatus:    b.HypertensionStatus,
		FastingGlucoseMgdl:    b.FastingGlucoseMgdl,
		HbA1cPct:              b.HbA1cPct,
		DiabetesStatus:        b.DiabetesStatus,
		TotalCholesterolMgdl:  b.TotalCholesterolMgdl,
		HDLMgdl:               b.HDLMgdl,
		LDLMgdl:               b.LDLMgdl,
		TriglyceidesMgdl:      b.TriglyceidesMgdl,
		CVDRisk10YrPct:        b.CVDRisk10YrPct,
		CVDRiskCategory:       b.CVDRiskCategory,
		OnAntihypertensive:    derefBool(b.OnAntihypertensive),
		OnAntidiabetic:        derefBool(b.OnAntidiabetic),
		OnStatin:              derefBool(b.OnStatin),
		TargetRisk:            b.TargetRisk,
		EGFR:                  b.EGFR,
		UACR:                  b.UACR,
		ClusterID:             b.ClusterID,
		DiagnosisCluster:      b.DiagnosisCluster,
		ClinicalGroup:         b.ClinicalGroup,
	}
}

func derefBool(p *bool) bool {
	if p != nil {
		return *p
	}
	return false
}
