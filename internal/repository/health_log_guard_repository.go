package repository

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

const (
	// idempotencyTTL menjaga key dedupe submission selama window double-tap mobile.
	// Sejalan docs/redis.md §3 (idempotency:{key}, TTL 5 menit).
	idempotencyTTL = 5 * time.Minute

	// submissionRateWindow & maxSubmissionsPerWindow adalah rate limit longgar untuk
	// endpoint input pasien (docs/api_guide.md §9) — cukup untuk pemakaian normal, tapi
	// mencegah app buggy/disusupi membanjiri backend.
	submissionRateWindow    = 1 * time.Minute
	maxSubmissionsPerWindow = 30

	// idempotencyPending menandai key yang sudah direservasi tapi insert-nya belum selesai.
	idempotencyPending = "PENDING"
)

var (
	ErrTooManySubmissions  = fmt.Errorf("terlalu banyak input dalam waktu singkat, coba lagi sebentar lagi")
	ErrIdempotencyInFlight = fmt.Errorf("request dengan Idempotency-Key yang sama sedang diproses")
)

// HealthLogGuardRepository memegang guard berbasis Redis untuk endpoint input health log:
// dedupe via Idempotency-Key dan rate limit per pasien.
type HealthLogGuardRepository struct {
	Redis *redis.Client
	Log   *zap.Logger
}

// CheckSubmissionRateLimit menaikkan counter submission pasien dan menolak bila melebihi
// batas dalam window. Pola sama dengan SessionRepository.CheckRateLimit.
func (r *HealthLogGuardRepository) CheckSubmissionRateLimit(ctx context.Context, patientID string) error {
	key := fmt.Sprintf("health_log_submit:%s", patientID)

	count, err := r.Redis.Incr(ctx, key).Result()
	if err != nil {
		return fmt.Errorf("incrementing health log submission counter: %w", err)
	}
	if count == 1 {
		r.Redis.Expire(ctx, key, submissionRateWindow)
	}
	if count > maxSubmissionsPerWindow {
		return ErrTooManySubmissions
	}
	return nil
}

// ReserveIdempotency mencoba mereservasi sebuah Idempotency-Key. Bila key belum pernah
// dipakai dalam window, mengembalikan isNew=true (pemanggil lanjut insert). Bila sudah ada,
// mengembalikan isNew=false beserta nilai tersimpan — berupa log id final, atau marker
// "PENDING" bila request pertama masih in-flight.
func (r *HealthLogGuardRepository) ReserveIdempotency(ctx context.Context, key string) (existingLogID string, isNew bool, err error) {
	redisKey := fmt.Sprintf("idempotency:%s", key)

	// SET key PENDING NX EX 300 — set hanya bila key belum ada. Bila sudah ada, go-redis
	// mengembalikan redis.Nil (bukan error sungguhan).
	err = r.Redis.SetArgs(ctx, redisKey, idempotencyPending, redis.SetArgs{
		Mode: "NX",
		TTL:  idempotencyTTL,
	}).Err()
	if err == nil {
		return "", true, nil
	}
	if !errors.Is(err, redis.Nil) {
		return "", false, fmt.Errorf("reserving idempotency key: %w", err)
	}

	stored, err := r.Redis.Get(ctx, redisKey).Result()
	if err != nil {
		// Key sempat ada saat SetNX tapi keburu kadaluarsa/terhapus sebelum Get; perlakukan
		// sebagai in-flight agar tidak terjadi double insert.
		return idempotencyPending, false, nil
	}
	return stored, false, nil
}

// CommitIdempotency menyimpan log id final ke key (menimpa marker PENDING) dan me-refresh TTL,
// sehingga request ulang dengan key yang sama mengembalikan id yang sama tanpa insert dobel.
func (r *HealthLogGuardRepository) CommitIdempotency(ctx context.Context, key, logID string) error {
	redisKey := fmt.Sprintf("idempotency:%s", key)
	if err := r.Redis.Set(ctx, redisKey, logID, idempotencyTTL).Err(); err != nil {
		return fmt.Errorf("committing idempotency key: %w", err)
	}
	return nil
}

// ReleaseIdempotency menghapus reservasi key. Dipakai bila insert gagal, agar client bisa
// retry dengan key yang sama tanpa nyangkut di marker PENDING.
func (r *HealthLogGuardRepository) ReleaseIdempotency(ctx context.Context, key string) error {
	redisKey := fmt.Sprintf("idempotency:%s", key)
	if err := r.Redis.Del(ctx, redisKey).Err(); err != nil {
		return fmt.Errorf("releasing idempotency key: %w", err)
	}
	return nil
}
