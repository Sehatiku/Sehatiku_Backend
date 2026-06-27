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
	"strings"
	"time"

	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

var ErrNIKAlreadyExists = errors.New("NIK sudah terdaftar")

type NakesRegistrationUseCase struct {
	DB         *gorm.DB
	NakesRepo  nakesRepo
	OCRGateway *ocr.KTPOCRGateway
	WhatsApp   *whatsapp.WhatsAppGateway
	Log        *zap.Logger
}

type nakesRepo interface {
	FindByNIK(db *gorm.DB, nik string) (*entity.Nakes, error)
	FindByUsername(db *gorm.DB, username string) (*entity.Nakes, error)
	Create(db *gorm.DB, entity *entity.Nakes) error
}

func (u *NakesRegistrationUseCase) ScanKTP(ctx context.Context, file multipart.File, filename string) (*model.KTPOCRResponse, error) {
	result, err := u.OCRGateway.ExtractKTP(ctx, file, filename)
	if err != nil {
		return nil, err
	}
	return convertOCRResult(result), nil
}

func (u *NakesRegistrationUseCase) RegisterNakes(ctx context.Context, faskesID string, req *model.NakesRegisterRequest) (*model.NakesRegisterResponse, error) {
	_, err := u.NakesRepo.FindByNIK(u.DB, req.NIK)
	if err == nil {
		return nil, ErrNIKAlreadyExists
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("checking NIK availability: %w", err)
	}

	_, err = u.NakesRepo.FindByUsername(u.DB, req.Username)
	if err == nil {
		return nil, ErrUsernameAlreadyExists
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, fmt.Errorf("checking username availability: %w", err)
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return nil, fmt.Errorf("hashing password: %w", err)
	}

	now := time.Now()
	nakes := &entity.Nakes{
		FaskesID:     faskesID,
		Username:     req.Username,
		PasswordHash: string(hash),
		FullName:     req.FullName,
		Role:         req.Role,
		NIK:          req.NIK,
		Alamat:       req.Alamat,
		PhoneNumber:  req.PhoneNumber,
		Status:       entity.NakesStatusActive,
		EnrolledAt:   now,
	}

	if err := u.NakesRepo.Create(u.DB, nakes); err != nil {
		return nil, fmt.Errorf("creating nakes: %w", err)
	}

	u.Log.Info("nakes registered", zap.String("nakes_id", nakes.ID), zap.String("faskes_id", faskesID))

	// Kirim kredensial login ke WA nakes secara fire-and-forget — kegagalan WA tidak
	// boleh menggagalkan registrasi yang sudah tersimpan (pola sama dengan login).
	go func(phone, name, username, password string) {
		if err := u.WhatsApp.SendRegistrationCredentials(context.Background(), phone, name, username, password); err != nil {
			u.Log.Warn("failed to send wa registration credentials", zap.String("nakes_id", nakes.ID), zap.Error(err))
		}
	}(nakes.PhoneNumber, nakes.FullName, nakes.Username, req.Password)

	return &model.NakesRegisterResponse{
		NakesID:    nakes.ID,
		FaskesID:   nakes.FaskesID,
		FullName:   nakes.FullName,
		Role:       nakes.Role,
		NIK:        nakes.NIK,
		EnrolledAt: nakes.EnrolledAt,
	}, nil
}

// convertOCRResult maps raw OCR API output to the frontend pre-fill DTO.
// - Converts tanggal_lahir from DD-MM-YYYY → YYYY-MM-DD
// - Converts jenis_kelamin LAKI-LAKI/PEREMPUAN → male/female
// - Builds a readable alamat from OCR address components
func convertOCRResult(r *ocr.KTPOCRResult) *model.KTPOCRResponse {
	dob := convertDate(r.TanggalLahir)
	sex := convertSex(r.JenisKelamin)
	alamat := buildAlamat(r.Alamat, r.RT, r.RW, r.Kelurahan, r.Kecamatan, r.Kota)

	return &model.KTPOCRResponse{
		NIK:         r.NIK,
		FullName:    r.Nama,
		DateOfBirth: dob,
		Sex:         sex,
		Alamat:      alamat,
	}
}

func convertDate(tanggalLahir string) string {
	t, err := time.Parse("02-01-2006", tanggalLahir)
	if err != nil {
		return tanggalLahir
	}
	return t.Format("2006-01-02")
}

func convertSex(jenisKelamin string) string {
	if strings.EqualFold(jenisKelamin, "LAKI-LAKI") {
		return "male"
	}
	return "female"
}

func buildAlamat(alamat, rt, rw, kelurahan, kecamatan, kota string) string {
	parts := []string{alamat}
	if rt != "" || rw != "" {
		parts = append(parts, "RT "+rt+"/RW "+rw)
	}
	if kelurahan != "" {
		parts = append(parts, "Kel. "+kelurahan)
	}
	if kecamatan != "" {
		parts = append(parts, "Kec. "+kecamatan)
	}
	if kota != "" {
		parts = append(parts, kota)
	}
	return strings.Join(parts, ", ")
}
