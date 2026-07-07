package repository

import (
	"fmt"
	"sehatiku-backend/internal/entity"

	"gorm.io/gorm"
)

type NakesRepository struct {
	Repository[entity.Nakes]
}

func (r *NakesRepository) FindByUsername(db *gorm.DB, username string) (*entity.Nakes, error) {
	var nakes entity.Nakes
	if err := db.Where("username = ?", username).First(&nakes).Error; err != nil {
		return nil, fmt.Errorf("finding nakes by username: %w", err)
	}
	return &nakes, nil
}

func (r *NakesRepository) FindByNIK(db *gorm.DB, nik string) (*entity.Nakes, error) {
	var nakes entity.Nakes
	if err := db.Where("nik = ?", nik).First(&nakes).Error; err != nil {
		return nil, fmt.Errorf("finding nakes by nik: %w", err)
	}
	return &nakes, nil
}

func (r *NakesRepository) FindByID(db *gorm.DB, id string) (*entity.Nakes, error) {
	var nakes entity.Nakes
	if err := db.Where("id = ?", id).First(&nakes).Error; err != nil {
		return nil, fmt.Errorf("finding nakes by id: %w", err)
	}
	return &nakes, nil
}

// FindByIDs mengembalikan nakes untuk sekumpulan id (satu query IN), dipakai untuk
// resolusi nama recorder secara batch (hindari N+1). ids kosong -> hasil kosong.
func (r *NakesRepository) FindByIDs(db *gorm.DB, ids []string) ([]entity.Nakes, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	var list []entity.Nakes
	if err := db.Where("id IN ?", ids).Find(&list).Error; err != nil {
		return nil, fmt.Errorf("finding nakes by ids: %w", err)
	}
	return list, nil
}

// FindByPhone mencari nakes berdasarkan phone_number yang sudah ternormalisasi
// (format internasional, mis. "62812..."). Dipakai saat cek nomor pengirim WA
// inbound agar nakes tidak mendapat balasan "belum terdaftar" yang seharusnya
// hanya untuk pasien/pendamping.
func (r *NakesRepository) FindByPhone(db *gorm.DB, phone string) (*entity.Nakes, error) {
	var nakes entity.Nakes
	if err := db.Where("phone_number = ? AND status = 'active'", phone).First(&nakes).Error; err != nil {
		return nil, fmt.Errorf("finding nakes by phone: %w", err)
	}
	return &nakes, nil
}

func (r *NakesRepository) FindByFaskesID(db *gorm.DB, faskesID string) ([]entity.Nakes, error) {
	var list []entity.Nakes
	if err := db.Where("faskes_id = ?", faskesID).Order("enrolled_at DESC").Find(&list).Error; err != nil {
		return nil, fmt.Errorf("finding nakes by faskes_id: %w", err)
	}
	return list, nil
}
