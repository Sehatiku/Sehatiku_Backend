package repository

import (
	"fmt"

	"gorm.io/gorm"
)

// PhoneRepository mengecek keunikan nomor telepon lintas tabel faskes, nakes,
// dan patients — bukan repository per-entity seperti yang lain karena query-nya
// memang menyentuh tiga tabel sekaligus.
type PhoneRepository struct{}

// InUse mengecek apakah nomor telepon (sudah dinormalisasi via
// helper.NormalizePhoneID) sudah dipakai oleh faskes, nakes, atau pasien
// (phone_number maupun companion_phone) mana pun, di semua status. Satu nomor
// WA hanya boleh terikat ke satu identitas — WAHealthLogUseCase menentukan
// identitas pengirim inbound murni dari nomor telepon (NakesRepository.FindByPhone /
// PatientRepository.FindByPhone / FindByCompanionPhone), jadi nomor yang dipakai
// dua identitas sekaligus akan membuat routing WA ambigu.
func (r *PhoneRepository) InUse(db *gorm.DB, phone string) (bool, error) {
	var count int64
	err := db.Raw(`
		SELECT
			(SELECT COUNT(*) FROM faskes WHERE phone_number = ?) +
			(SELECT COUNT(*) FROM nakes WHERE phone_number = ?) +
			(SELECT COUNT(*) FROM patients WHERE phone_number = ? OR companion_phone = ?)
	`, phone, phone, phone, phone).Scan(&count).Error
	if err != nil {
		return false, fmt.Errorf("checking phone uniqueness: %w", err)
	}
	return count > 0, nil
}
