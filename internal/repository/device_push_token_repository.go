// internal/repository/device_push_token_repository.go
package repository

import (
	"fmt"
	"time"

	"sehatiku-backend/internal/entity"

	"gorm.io/gorm"
)

// DevicePushTokenRepository mengelola tabel `device_push_tokens`. Registrasi token bersifat
// upsert by token (UNIQUE constraint): mendaftarkan token yang sama dari patient_id berbeda
// memindahkan kepemilikan, bukan gagal — menangani kasus app uninstall lalu install ulang
// di HP lain dengan akun berbeda.
type DevicePushTokenRepository struct{}

// Upsert mendaftarkan/memperbarui satu device token milik pasien. Token yang sudah ada
// (dari pasien mana pun) dipindah kepemilikan ke patientID dan diaktifkan kembali.
func (r *DevicePushTokenRepository) Upsert(db *gorm.DB, patientID, platform, token string) error {
	now := time.Now()
	err := db.Exec(`
		INSERT INTO device_push_tokens (id, patient_id, platform, token, is_active, created_at, updated_at)
		VALUES (gen_random_uuid(), ?, ?, ?, true, ?, ?)
		ON CONFLICT (token) DO UPDATE SET
			patient_id = EXCLUDED.patient_id,
			platform   = EXCLUDED.platform,
			is_active  = true,
			updated_at = EXCLUDED.updated_at
	`, patientID, platform, token, now, now).Error
	if err != nil {
		return fmt.Errorf("upserting device push token for patient %s: %w", patientID, err)
	}
	return nil
}

// DeactivateByToken menonaktifkan satu token milik pasien (dipanggil saat logout/deregister
// eksplisit). Idempoten: token tidak ditemukan atau bukan milik pasien ini tetap tidak error
// (RowsAffected 0 diperbolehkan — caller tidak perlu menangani error khusus).
func (r *DevicePushTokenRepository) DeactivateByToken(db *gorm.DB, patientID, token string) error {
	if err := db.Model(&entity.DevicePushToken{}).
		Where("patient_id = ? AND token = ?", patientID, token).
		Update("is_active", false).Error; err != nil {
		return fmt.Errorf("deactivating device push token for patient %s: %w", patientID, err)
	}
	return nil
}

// FindActiveByPatient mengembalikan semua token aktif milik pasien (multi-device: bisa lebih
// dari satu).
func (r *DevicePushTokenRepository) FindActiveByPatient(db *gorm.DB, patientID string) ([]string, error) {
	var tokens []string
	if err := db.Model(&entity.DevicePushToken{}).
		Where("patient_id = ? AND is_active = true", patientID).
		Pluck("token", &tokens).Error; err != nil {
		return nil, fmt.Errorf("finding active device push tokens for patient %s: %w", patientID, err)
	}
	return tokens, nil
}

// DeactivateTokens menonaktifkan sekumpulan token sekaligus — dipakai saat FCM menandai
// token sebagai invalid/unregistered, agar tidak dicoba kirim lagi di request berikutnya.
func (r *DevicePushTokenRepository) DeactivateTokens(db *gorm.DB, tokens []string) error {
	if len(tokens) == 0 {
		return nil
	}
	if err := db.Model(&entity.DevicePushToken{}).
		Where("token IN ?", tokens).
		Update("is_active", false).Error; err != nil {
		return fmt.Errorf("deactivating invalid device push tokens: %w", err)
	}
	return nil
}
