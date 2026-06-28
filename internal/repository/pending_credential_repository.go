package repository

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"sehatiku-backend/internal/helper"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"
)

// PendingCredentialDefaultTTL adalah berapa lama kredensial yang menunggu warm-up
// disimpan di Redis. Cukup panjang supaya pasien/pendamping/nakes sempat menghubungi
// bot setelah faskes meneruskan link, tapi tetap terbatas karena ini menyimpan password
// sementara (lihat catatan keamanan di bawah). Setelah lewat TTL, kredensial dianggap
// kedaluwarsa — faskes tetap memegangnya dari response registrasi sebagai cadangan.
const PendingCredentialDefaultTTL = 72 * time.Hour

// PendingCredential adalah kredensial login yang menunggu untuk dikirim via WhatsApp
// SETELAH penerima menghubungi bot lebih dulu (warm-up). Disimpan sementara di Redis,
// di-keyed oleh nomor telepon ternormalisasi penerima.
//
// KEAMANAN: ini satu-satunya tempat password hidup (sementara) di luar response
// registrasi — `notifications.payload` di Postgres sengaja TIDAK menyimpan password.
// Eksposur dibatasi TTL dan key dihapus tepat setelah pengiriman berhasil. Password yang
// sama sudah dikembalikan ke faskes di response registrasi, jadi tidak ada secret baru
// yang tercipta — hanya disalin sebentar agar bot bisa mengirimkannya saat penerima masuk.
type PendingCredential struct {
	Role           string `json:"role"`            // "patient" | "companion" | "nakes"
	RecipientName  string `json:"recipient_name"`  // nama penerima pesan (pasien/pendamping/nakes)
	PatientName    string `json:"patient_name"`    // hanya untuk role "companion"
	Username       string `json:"username"`        // username akun yang dikirim
	Password       string `json:"password"`        // password plaintext (lihat catatan keamanan)
	NotificationID string `json:"notification_id"` // baris notifications untuk di-update jadi "sent"
}

// PendingCredentialRepository menyimpan kredensial menunggu warm-up di Redis. Tidak
// meng-embed Repository[T] generic (yang untuk GORM) — backend datanya Redis, sama
// seperti SessionRepository (lihat docs/redis.md §9).
type PendingCredentialRepository struct {
	Redis *redis.Client
	Log   *zap.Logger
}

func pendingCredentialKey(phone string) string {
	return fmt.Sprintf("pending_credential:%s", helper.NormalizePhoneID(phone))
}

// Stash menyimpan kredensial menunggu warm-up untuk satu nomor, dengan TTL.
func (r *PendingCredentialRepository) Stash(ctx context.Context, phone string, data PendingCredential, ttl time.Duration) error {
	encoded, err := json.Marshal(data)
	if err != nil {
		return fmt.Errorf("marshalling pending credential: %w", err)
	}
	if err := r.Redis.Set(ctx, pendingCredentialKey(phone), encoded, ttl).Err(); err != nil {
		return fmt.Errorf("stashing pending credential in redis: %w", err)
	}
	return nil
}

// Get mengambil kredensial menunggu warm-up untuk satu nomor. Mengembalikan (nil, nil)
// bila tidak ada — itu kondisi normal (mayoritas pesan masuk bukan warm-up), bukan error.
func (r *PendingCredentialRepository) Get(ctx context.Context, phone string) (*PendingCredential, error) {
	raw, err := r.Redis.Get(ctx, pendingCredentialKey(phone)).Result()
	if err == redis.Nil {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("getting pending credential from redis: %w", err)
	}

	var data PendingCredential
	if err := json.Unmarshal([]byte(raw), &data); err != nil {
		return nil, fmt.Errorf("unmarshalling pending credential: %w", err)
	}
	return &data, nil
}

// Delete menghapus kredensial menunggu warm-up — dipanggil setelah pengiriman berhasil
// supaya tidak terkirim dua kali.
func (r *PendingCredentialRepository) Delete(ctx context.Context, phone string) error {
	if err := r.Redis.Del(ctx, pendingCredentialKey(phone)).Err(); err != nil {
		return fmt.Errorf("deleting pending credential from redis: %w", err)
	}
	return nil
}
