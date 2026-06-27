package usecase

import (
	"context"
	"errors"
	"fmt"
	"sehatiku-backend/internal/helper"
	"sehatiku-backend/internal/model"
	"sehatiku-backend/internal/repository"

	"go.uber.org/zap"
)

type TokenUseCase struct {
	SessionRepo *repository.SessionRepository
	JWT         *helper.JWTHelper
	Log         *zap.Logger
}

func (u *TokenUseCase) Refresh(ctx context.Context, req *model.RefreshTokenRequest) (*model.TokenResponse, error) {
	data, newRefreshToken, err := u.SessionRepo.ValidateAndRotate(ctx, req.RefreshToken)
	if err != nil {
		if errors.Is(err, repository.ErrRefreshTokenReused) {
			u.Log.Warn("refresh token reuse detected", zap.String("role", data.Role))
		}
		return nil, fmt.Errorf("rotating refresh token: %w", err)
	}

	var accessToken string
	switch data.Role {
	case "faskes":
		accessToken, err = u.JWT.GenerateFaskesToken(data.UserID)
	case "nakes":
		accessToken, err = u.JWT.GenerateNakesToken(data.UserID, data.FaskesID, data.NakesRole)
	case "patient":
		accessToken, err = u.JWT.GeneratePatientToken(data.UserID, data.FaskesID)
	default:
		return nil, fmt.Errorf("unknown role in refresh token: %s", data.Role)
	}
	if err != nil {
		return nil, fmt.Errorf("generating new access token: %w", err)
	}

	return &model.TokenResponse{
		AccessToken:  accessToken,
		RefreshToken: newRefreshToken,
		ExpiresIn:    helper.AccessTokenTTLSeconds(),
	}, nil
}

func (u *TokenUseCase) Logout(ctx context.Context, req *model.LogoutRequest, role, userID string) error {
	if err := u.SessionRepo.Revoke(ctx, req.RefreshToken, role, userID); err != nil {
		return fmt.Errorf("revoking session: %w", err)
	}
	return nil
}
