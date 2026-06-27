package usecase

import (
	"context"
	"errors"
	"fmt"
	"mime/multipart"
	"sehatiku-backend/internal/entity"
	"sehatiku-backend/internal/gateway/ocr"
	"sehatiku-backend/internal/model"
	"time"

	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type PatientRegistrationUseCase struct {
	DB          *gorm.DB
	PatientRepo patientRepo
	OCRGateway  *ocr.KTPOCRGateway
	Log         *zap.Logger
}

type patientRepo interface {
	FindByNIK(db *gorm.DB, nik string) (*entity.Patient, error)
	FindByUsername(db *gorm.DB, username string) (*entity.Patient, error)
	Create(db *gorm.DB, entity *entity.Patient) error
}

func (u *PatientRegistrationUseCase) ScanKTP(ctx context.Context, file multipart.File, filename string) (*model.KTPOCRResponse, error) {
	result, err := u.OCRGateway.ExtractKTP(ctx, file, filename)
	if err != nil {
		return nil, err
	}
	return convertOCRResult(result), nil
}

func (u *PatientRegistrationUseCase) RegisterPatient(ctx context.Context, faskesID, nakesID string, req *model.PatientRegisterRequest) (*model.PatientRegisterResponse, error) {
	_, err := u.PatientRepo.FindByNIK(u.DB, req.NIK)
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
		AssignedNakesID: nakesID,
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

	return &model.PatientRegisterResponse{
		PatientID:   patient.ID,
		FaskesID:    patient.FaskesID,
		FullName:    patient.FullName,
		NIK:         patient.NIK,
		DiseaseType: patient.DiseaseType,
		EnrolledAt:  patient.EnrolledAt,
	}, nil
}
