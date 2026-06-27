package usecase

import (
	"context"
	"errors"
	"fmt"
	"sehatiku-backend/internal/gateway/whatsapp"
	"sehatiku-backend/internal/helper"
	"sehatiku-backend/internal/model"
	"sehatiku-backend/internal/repository"

	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

type PatientAuthUseCase struct {
	DB          *gorm.DB
	PatientRepo *repository.PatientRepository
	SessionRepo *repository.SessionRepository
	JWT         *helper.JWTHelper
	WhatsApp    *whatsapp.WhatsAppGateway
	Log         *zap.Logger
}

func (u *PatientAuthUseCase) Login(ctx context.Context, req *model.PatientLoginRequest) (*model.PatientLoginResponse, error) {
	if err := u.SessionRepo.CheckRateLimit(ctx, "patient", req.Username); err != nil {
		return nil, err
	}

	patient, err := u.PatientRepo.FindByUsername(u.DB, req.Username)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrInvalidCredentials
		}
		return nil, fmt.Errorf("finding patient: %w", err)
	}

	if patient.Status != "active" {
		return nil, ErrAccountInactive
	}

	if err := bcrypt.CompareHashAndPassword([]byte(patient.PasswordHash), []byte(req.Password)); err != nil {
		return nil, ErrInvalidCredentials
	}

	if err := u.SessionRepo.ResetRateLimit(ctx, "patient", req.Username); err != nil {
		u.Log.Warn("failed to reset rate limit", zap.String("username", req.Username), zap.Error(err))
	}

	// Patient: single-device — revoke semua sesi lama sebelum buat yang baru (redis.md §2)
	if err := u.SessionRepo.RevokeAllForUser(ctx, "patient", patient.ID); err != nil {
		return nil, fmt.Errorf("revoking previous patient sessions: %w", err)
	}

	accessToken, err := u.JWT.GeneratePatientToken(patient.ID, patient.FaskesID)
	if err != nil {
		return nil, fmt.Errorf("generating access token: %w", err)
	}

	refreshToken, err := u.SessionRepo.IssueRefreshToken(ctx, repository.RefreshTokenData{
		UserID:   patient.ID,
		Role:     "patient",
		FaskesID: patient.FaskesID,
	}, repository.RefreshTokenTTL("patient"))
	if err != nil {
		return nil, fmt.Errorf("issuing refresh token: %w", err)
	}

	go func() {
		if err := u.WhatsApp.SendLoginNotification(context.Background(), patient.PhoneNumber, patient.FullName); err != nil {
			u.Log.Warn("failed to send wa login notification", zap.String("patient_id", patient.ID), zap.Error(err))
		}
	}()

	return &model.PatientLoginResponse{
		Token: model.TokenResponse{
			AccessToken:  accessToken,
			RefreshToken: refreshToken,
			ExpiresIn:    helper.AccessTokenTTLSeconds(),
		},
		PatientID: patient.ID,
		FaskesID:  patient.FaskesID,
		FullName:  patient.FullName,
	}, nil
}
