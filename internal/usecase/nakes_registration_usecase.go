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
	"strings"
	"time"

	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

var ErrNIKAlreadyExists = errors.New("NIK sudah terdaftar")

type NakesRegistrationUseCase struct {
	DB                *gorm.DB
	NakesRepo         nakesRepo
	NotificationRepo  notificationRepo
	PendingCredential pendingCredentialStasher
	OCRGateway        *ocr.KTPOCRGateway
	WhatsApp          *whatsapp.WhatsAppGateway
	Log               *zap.Logger
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

	// Alur warm-up (sama seperti registrasi pasien): WhatsApp memblokir kontak baru
	// (error 463), jadi kredensial tidak dikirim duluan. Catat audit `queued`, simpan
	// kredensial menunggu di Redis, dan kembalikan link wa.me supaya nakes menghubungi bot
	// dahulu. Faskes tetap memegang kredensial dari response sebagai cadangan terjamin.
	botPhone := u.WhatsApp.BotPhone()
	warmup := model.WAWarmupStatus{BotPhone: botPhone, Status: warmupStatusPending}
	if botPhone == "" {
		warmup.Status = warmupStatusUnavailable
	}
	warmup.NakesLink = u.prepareWarmup(ctx, nakes.ID, nakes.PhoneNumber, nakes.FullName, nakes.Username, req.Password, botPhone)
	warmup.NakesMessage = helper.BuildWarmupShareMessage(
		entity.RecipientRoleNakes, nakes.FullName, "", nakes.Username, warmup.NakesLink)

	return &model.NakesRegisterResponse{
		NakesID:    nakes.ID,
		FaskesID:   nakes.FaskesID,
		FullName:   nakes.FullName,
		Role:       nakes.Role,
		NIK:        nakes.NIK,
		EnrolledAt: nakes.EnrolledAt,
		Credentials: model.NakesCredentials{
			Username: nakes.Username,
			Password: req.Password,
		},
		WAWarmup: warmup,
	}, nil
}

// prepareWarmup mencatat audit notifications (queued, TANPA password), menyimpan kredensial
// menunggu warm-up di Redis, dan mengembalikan link wa.me. Kegagalan DB/Redis di-log tapi
// tidak menggagalkan registrasi (partial failure, be_implementation §6).
func (u *NakesRegistrationUseCase) prepareWarmup(
	ctx context.Context,
	nakesID, recipientPhone, recipientName, username, password, botPhone string,
) string {
	payload, _ := json.Marshal(map[string]string{
		"username":       username,
		"recipient_name": recipientName,
	})

	nid := nakesID
	notif := &entity.Notification{
		NakesID:        &nid,
		RecipientPhone: recipientPhone,
		RecipientRole:  entity.RecipientRoleNakes,
		MessageType:    entity.MessageTypeCredentialBlast,
		Channel:        "whatsapp",
		Payload:        string(payload),
		Status:         entity.NotificationStatusQueued,
		QueuedAt:       time.Now(),
	}
	if err := u.NotificationRepo.Create(u.DB, notif); err != nil {
		u.Log.Warn("failed to record credential notification",
			zap.String("nakes_id", nakesID), zap.Error(err))
	}

	if err := u.PendingCredential.Stash(ctx, recipientPhone, repository.PendingCredential{
		Role:           entity.RecipientRoleNakes,
		RecipientName:  recipientName,
		Username:       username,
		Password:       password,
		NotificationID: notif.ID,
	}, repository.PendingCredentialDefaultTTL); err != nil {
		u.Log.Warn("failed to stash pending credential for warm-up",
			zap.String("nakes_id", nakesID), zap.String("phone", helper.MaskPhone(recipientPhone)), zap.Error(err))
	}

	return helper.BuildWAMeLink(botPhone, waWarmupPrefillText)
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
