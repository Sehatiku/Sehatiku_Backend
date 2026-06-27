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

type NakesAuthUseCase struct {
	DB          *gorm.DB
	NakesRepo   *repository.NakesRepository
	SessionRepo *repository.SessionRepository
	JWT         *helper.JWTHelper
	WhatsApp    *whatsapp.WhatsAppGateway
	Log         *zap.Logger
}

func (u *NakesAuthUseCase) Login(ctx context.Context, req *model.NakesLoginRequest) (*model.NakesLoginResponse, error) {
	if err := u.SessionRepo.CheckRateLimit(ctx, "nakes", req.Username); err != nil {
		return nil, err
	}

	nakes, err := u.NakesRepo.FindByUsername(u.DB, req.Username)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrInvalidCredentials
		}
		return nil, fmt.Errorf("finding nakes: %w", err)
	}

	if nakes.Status != "active" {
		return nil, ErrAccountInactive
	}

	if err := bcrypt.CompareHashAndPassword([]byte(nakes.PasswordHash), []byte(req.Password)); err != nil {
		return nil, ErrInvalidCredentials
	}

	if err := u.SessionRepo.ResetRateLimit(ctx, "nakes", req.Username); err != nil {
		u.Log.Warn("failed to reset rate limit", zap.String("username", req.Username), zap.Error(err))
	}

	accessToken, err := u.JWT.GenerateNakesToken(nakes.ID, nakes.FaskesID, nakes.Role)
	if err != nil {
		return nil, fmt.Errorf("generating access token: %w", err)
	}

	// Nakes: multi-device diperbolehkan — tidak revoke sesi lama (redis.md §2)
	refreshToken, err := u.SessionRepo.IssueRefreshToken(ctx, repository.RefreshTokenData{
		UserID:    nakes.ID,
		Role:      "nakes",
		FaskesID:  nakes.FaskesID,
		NakesRole: nakes.Role,
	}, repository.RefreshTokenTTL("nakes"))
	if err != nil {
		return nil, fmt.Errorf("issuing refresh token: %w", err)
	}

	go func() {
		if err := u.WhatsApp.SendLoginNotification(context.Background(), nakes.PhoneNumber, nakes.FullName); err != nil {
			u.Log.Warn("failed to send wa login notification", zap.String("nakes_id", nakes.ID), zap.Error(err))
		}
	}()

	return &model.NakesLoginResponse{
		Token: model.TokenResponse{
			AccessToken:  accessToken,
			RefreshToken: refreshToken,
			ExpiresIn:    helper.AccessTokenTTLSeconds(),
		},
		NakesID:  nakes.ID,
		FaskesID: nakes.FaskesID,
		FullName: nakes.FullName,
		Role:     nakes.Role,
	}, nil
}
