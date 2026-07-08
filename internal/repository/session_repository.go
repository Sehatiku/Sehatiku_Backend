package repository

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

const (
	refreshTokenNakesTTL   = 7 * 24 * time.Hour
	refreshTokenPatientTTL = 60 * 24 * time.Hour
	loginRateLimitWindow   = 15 * time.Minute
	maxLoginAttempts       = 5
)

var (
	ErrTooManyLoginAttempts = fmt.Errorf("terlalu banyak percobaan login, coba lagi dalam 15 menit")
	ErrRefreshTokenInvalid  = fmt.Errorf("refresh token tidak valid atau sudah kadaluarsa")
	ErrRefreshTokenReused   = fmt.Errorf("refresh token digunakan ulang — semua sesi direvoke")
)

type SessionRepository struct {
	Redis *redis.Client
	Log   *zap.Logger
}

// RefreshTokenData berisi data yang disimpan di Redis untuk setiap refresh token.
type RefreshTokenData struct {
	UserID    string
	Role      string
	FaskesID  string
	NakesRole string // diisi hanya untuk role="nakes": dokter|kader|admin
}

func RefreshTokenTTL(role string) time.Duration {
	if role == "patient" {
		return refreshTokenPatientTTL
	}
	return refreshTokenNakesTTL
}

// IssueRefreshToken membuat opaque refresh token baru dan menyimpannya di Redis.
// nakesRole hanya relevan untuk role="nakes"; untuk role lain, isi string kosong.
func (r *SessionRepository) IssueRefreshToken(ctx context.Context, data RefreshTokenData, ttl time.Duration) (string, error) {
	raw, err := generateOpaqueToken()
	if err != nil {
		return "", fmt.Errorf("generating refresh token: %w", err)
	}

	hash := hashToken(raw)
	tokenKey := fmt.Sprintf("refresh_token:%s", hash)
	sessionSetKey := fmt.Sprintf("user_sessions:%s:%s", data.Role, data.UserID)

	fields := map[string]any{
		"user_id":    data.UserID,
		"role":       data.Role,
		"faskes_id":  data.FaskesID,
		"nakes_role": data.NakesRole,
		"issued_at":  time.Now().Unix(),
	}

	pipe := r.Redis.Pipeline()
	pipe.HSet(ctx, tokenKey, fields)
	pipe.Expire(ctx, tokenKey, ttl)
	pipe.SAdd(ctx, sessionSetKey, hash)

	if _, err := pipe.Exec(ctx); err != nil {
		return "", fmt.Errorf("storing refresh token in redis: %w", err)
	}
	return raw, nil
}

// ValidateAndRotate memvalidasi refresh token, merotasi ke token baru, dan mengembalikan data sesi.
func (r *SessionRepository) ValidateAndRotate(ctx context.Context, rawToken string) (data RefreshTokenData, newToken string, err error) {
	hash := hashToken(rawToken)
	tokenKey := fmt.Sprintf("refresh_token:%s", hash)
	revokedKey := fmt.Sprintf("revoked_token:%s", hash)

	reuseExists, err := r.Redis.Exists(ctx, revokedKey).Result()
	if err != nil {
		return data, "", fmt.Errorf("checking revoked token: %w", err)
	}
	if reuseExists > 0 {
		stored, _ := r.Redis.HGetAll(ctx, revokedKey).Result()
		if uid := stored["user_id"]; uid != "" {
			_ = r.RevokeAllForUser(ctx, stored["role"], uid)
		}
		return data, "", ErrRefreshTokenReused
	}

	stored, err := r.Redis.HGetAll(ctx, tokenKey).Result()
	if err != nil || len(stored) == 0 {
		return data, "", ErrRefreshTokenInvalid
	}

	data = RefreshTokenData{
		UserID:    stored["user_id"],
		Role:      stored["role"],
		FaskesID:  stored["faskes_id"],
		NakesRole: stored["nakes_role"],
	}

	ttl, _ := r.Redis.TTL(ctx, tokenKey).Result()
	sessionSetKey := fmt.Sprintf("user_sessions:%s:%s", data.Role, data.UserID)

	pipe := r.Redis.Pipeline()
	pipe.Del(ctx, tokenKey)
	pipe.SRem(ctx, sessionSetKey, hash)
	pipe.HSet(ctx, revokedKey, map[string]any{
		"user_id": data.UserID,
		"role":    data.Role,
	})
	if ttl > 0 {
		pipe.Expire(ctx, revokedKey, ttl)
	}
	if _, err = pipe.Exec(ctx); err != nil {
		return data, "", fmt.Errorf("rotating refresh token: %w", err)
	}

	newToken, err = r.IssueRefreshToken(ctx, data, RefreshTokenTTL(data.Role))
	if err != nil {
		return data, "", fmt.Errorf("issuing new refresh token: %w", err)
	}
	return data, newToken, nil
}

// Revoke menghapus satu refresh token dari Redis.
func (r *SessionRepository) Revoke(ctx context.Context, rawToken, role, userID string) error {
	hash := hashToken(rawToken)
	tokenKey := fmt.Sprintf("refresh_token:%s", hash)
	sessionSetKey := fmt.Sprintf("user_sessions:%s:%s", role, userID)

	pipe := r.Redis.Pipeline()
	pipe.Del(ctx, tokenKey)
	pipe.SRem(ctx, sessionSetKey, hash)
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("revoking token in redis: %w", err)
	}
	return nil
}

// RevokeAllForUser menghapus semua sesi aktif milik user tertentu.
func (r *SessionRepository) RevokeAllForUser(ctx context.Context, role, userID string) error {
	sessionSetKey := fmt.Sprintf("user_sessions:%s:%s", role, userID)

	hashes, err := r.Redis.SMembers(ctx, sessionSetKey).Result()
	if err != nil {
		return fmt.Errorf("getting user sessions: %w", err)
	}

	pipe := r.Redis.Pipeline()
	for _, h := range hashes {
		pipe.Del(ctx, fmt.Sprintf("refresh_token:%s", h))
	}
	pipe.Del(ctx, sessionSetKey)
	if _, err := pipe.Exec(ctx); err != nil {
		return fmt.Errorf("revoking all sessions: %w", err)
	}
	return nil
}

// CheckRateLimit menaikkan counter login gagal dan menolak jika melebihi batas.
func (r *SessionRepository) CheckRateLimit(ctx context.Context, role, identifier string) error {
	key := fmt.Sprintf("login_attempts:%s:%s", role, identifier)

	count, err := r.Redis.Incr(ctx, key).Result()
	if err != nil {
		return fmt.Errorf("incrementing rate limit counter: %w", err)
	}
	if count == 1 {
		r.Redis.Expire(ctx, key, loginRateLimitWindow)
	}
	if count > maxLoginAttempts {
		return ErrTooManyLoginAttempts
	}
	return nil
}

// ResetRateLimit menghapus counter login gagal setelah login berhasil.
func (r *SessionRepository) ResetRateLimit(ctx context.Context, role, identifier string) error {
	key := fmt.Sprintf("login_attempts:%s:%s", role, identifier)
	return r.Redis.Del(ctx, key).Err()
}

func generateOpaqueToken() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func hashToken(raw string) string {
	sum := sha256.Sum256([]byte(raw))
	return hex.EncodeToString(sum[:])
}
