package usecase

import (
	"context"
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

// ErrAssignedNakesInvalid dikembalikan bila assigned_nakes_id pada request tidak
// merujuk ke nakes mana pun, atau merujuk ke nakes milik faskes lain (isolasi tenant).
var ErrAssignedNakesInvalid = errors.New("assigned_nakes_id tidak valid atau bukan milik faskes ini")

type PatientRegistrationUseCase struct {
	DB          *gorm.DB
	PatientRepo patientRepo
	NakesRepo   patientRegNakesRepo
	OCRGateway  *ocr.KTPOCRGateway
	WhatsApp    *whatsapp.WhatsAppGateway
	Log         *zap.Logger
}

type patientRepo interface {
	FindByNIK(db *gorm.DB, nik string) (*entity.Patient, error)
	FindByUsername(db *gorm.DB, username string) (*entity.Patient, error)
	Create(db *gorm.DB, entity *entity.Patient) error
}

type patientRegNakesRepo interface {
	FindByID(db *gorm.DB, id string) (*entity.Nakes, error)
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

	// Kirim kredensial login ke WA pasien dan pendamping secara fire-and-forget —
	// kegagalan WA tidak boleh menggagalkan registrasi yang sudah tersimpan.
	go func(p entity.Patient, password string) {
		if err := u.WhatsApp.SendRegistrationCredentials(context.Background(), p.PhoneNumber, p.FullName, p.Username, password); err != nil {
			u.Log.Warn("failed to send wa registration credentials to patient", zap.String("patient_id", p.ID), zap.Error(err))
		}
		if p.CompanionPhone == "" {
			return
		}
		if err := u.WhatsApp.SendCompanionRegistrationCredentials(context.Background(), p.CompanionPhone, p.CompanionName, p.FullName, p.Username, password); err != nil {
			u.Log.Warn("failed to send wa registration credentials to companion", zap.String("patient_id", p.ID), zap.Error(err))
		}
	}(*patient, req.Password)

	return &model.PatientRegisterResponse{
		PatientID:   patient.ID,
		FaskesID:    patient.FaskesID,
		FullName:    patient.FullName,
		NIK:         patient.NIK,
		DiseaseType: patient.DiseaseType,
		EnrolledAt:  patient.EnrolledAt,
	}, nil
}
