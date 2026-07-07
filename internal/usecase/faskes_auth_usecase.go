package usecase

import (
	"context"
	"errors"
	"fmt"
	"sehatiku-backend/internal/entity"
	"sehatiku-backend/internal/helper"
	"sehatiku-backend/internal/model"
	"sehatiku-backend/internal/repository"

	"go.uber.org/zap"
	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

var (
	ErrInvalidCredentials    = errors.New("username atau password salah")
	ErrAccountInactive       = errors.New("akun tidak aktif")
	ErrUsernameAlreadyExists = errors.New("username sudah digunakan")
)

type FaskesAuthUseCase struct {
	DB          *gorm.DB
	FaskesRepo  *repository.FaskesRepository
	SessionRepo *repository.SessionRepository
	JWT         *helper.JWTHelper
	Log         *zap.Logger
}

func (u *FaskesAuthUseCase) Register(ctx context.Context, req *model.FaskesRegisterRequest) error {
	_, err := u.FaskesRepo.FindByUsername(u.DB, req.Username)
	if err == nil {
		return ErrUsernameAlreadyExists
	}
	if !errors.Is(err, gorm.ErrRecordNotFound) {
		return fmt.Errorf("checking username availability: %w", err)
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hashing password: %w", err)
	}

	faskes := &entity.Faskes{
		Name:         req.Name,
		Type:         req.Type,
		Address:      req.Address,
		Region:       req.Region,
		Username:     req.Username,
		PasswordHash: string(hash),
		// Normalisasi ke 62... agar konsisten dengan nomor WA (warm-up credential &
		// notifikasi). Lihat helper.NormalizePhoneID.
		PhoneNumber:  helper.NormalizePhoneID(req.PhoneNumber),
		Status:       "active",
	}

	if err := u.FaskesRepo.Create(u.DB, faskes); err != nil {
		return fmt.Errorf("creating faskes: %w", err)
	}
	return nil
}

func (u *FaskesAuthUseCase) Login(ctx context.Context, req *model.FaskesLoginRequest) (*model.FaskesLoginResponse, error) {
	if err := u.SessionRepo.CheckRateLimit(ctx, "faskes", req.Username); err != nil {
		return nil, err
	}

	faskes, err := u.FaskesRepo.FindByUsername(u.DB, req.Username)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrInvalidCredentials
		}
		return nil, fmt.Errorf("finding faskes: %w", err)
	}

	if faskes.Status != "active" {
		return nil, ErrAccountInactive
	}

	if err := bcrypt.CompareHashAndPassword([]byte(faskes.PasswordHash), []byte(req.Password)); err != nil {
		return nil, ErrInvalidCredentials
	}

	if err := u.SessionRepo.ResetRateLimit(ctx, "faskes", req.Username); err != nil {
		u.Log.Warn("failed to reset rate limit", zap.String("username", req.Username), zap.Error(err))
	}

	accessToken, err := u.JWT.GenerateFaskesToken(faskes.ID)
	if err != nil {
		return nil, fmt.Errorf("generating access token: %w", err)
	}

	refreshToken, err := u.SessionRepo.IssueRefreshToken(ctx, repository.RefreshTokenData{
		UserID:   faskes.ID,
		Role:     "faskes",
		FaskesID: faskes.ID,
	}, repository.RefreshTokenTTL("faskes"))
	if err != nil {
		return nil, fmt.Errorf("issuing refresh token: %w", err)
	}

	return &model.FaskesLoginResponse{
		Token: model.TokenResponse{
			AccessToken:  accessToken,
			RefreshToken: refreshToken,
			ExpiresIn:    helper.AccessTokenTTLSeconds(),
		},
		FaskesID: faskes.ID,
		Name:     faskes.Name,
	}, nil
}
