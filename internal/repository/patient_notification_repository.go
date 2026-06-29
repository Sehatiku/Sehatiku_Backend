package repository

import (
	"fmt"
	"time"

	"sehatiku-backend/internal/entity"

	"gorm.io/gorm"
)

// PatientNotificationRepository mengelola inbox in-app pasien (tabel patient_notifications).
// Operasi mark-read selalu di-scope dengan patient_id agar pasien tidak bisa menyentuh
// notifikasi milik pasien lain (ownership safety).
type PatientNotificationRepository struct {
	Repository[entity.PatientNotification]
}

// FindByPatientID mengembalikan seluruh notifikasi inbox milik pasien, terbaru dulu.
func (r *PatientNotificationRepository) FindByPatientID(db *gorm.DB, patientID string) ([]entity.PatientNotification, error) {
	var rows []entity.PatientNotification
	if err := db.Where("patient_id = ?", patientID).
		Order("created_at DESC").
		Find(&rows).Error; err != nil {
		return nil, fmt.Errorf("finding patient notifications for patient %s: %w", patientID, err)
	}
	return rows, nil
}

// CountUnread menghitung notifikasi yang belum dibaca (read_at IS NULL) milik pasien.
func (r *PatientNotificationRepository) CountUnread(db *gorm.DB, patientID string) (int64, error) {
	var count int64
	if err := db.Model(&entity.PatientNotification{}).
		Where("patient_id = ? AND read_at IS NULL", patientID).
		Count(&count).Error; err != nil {
		return 0, fmt.Errorf("counting unread notifications for patient %s: %w", patientID, err)
	}
	return count, nil
}

// MarkRead menandai satu notifikasi milik pasien sebagai sudah dibaca. Idempoten: baris yang
// sudah terbaca tidak diubah lagi. RowsAffected=0 dipakai caller untuk membedakan not-found.
func (r *PatientNotificationRepository) MarkRead(db *gorm.DB, id, patientID string) (int64, error) {
	result := db.Model(&entity.PatientNotification{}).
		Where("id = ? AND patient_id = ?", id, patientID).
		Where("read_at IS NULL").
		Update("read_at", time.Now())
	if result.Error != nil {
		return 0, fmt.Errorf("marking notification %s read: %w", id, result.Error)
	}
	return result.RowsAffected, nil
}

// ExistsForPatient memastikan sebuah notifikasi memang milik pasien (untuk membedakan
// 404 not-found dari kondisi sudah-terbaca pada MarkRead yang idempoten).
func (r *PatientNotificationRepository) ExistsForPatient(db *gorm.DB, id, patientID string) (bool, error) {
	var count int64
	if err := db.Model(&entity.PatientNotification{}).
		Where("id = ? AND patient_id = ?", id, patientID).
		Count(&count).Error; err != nil {
		return false, fmt.Errorf("checking notification %s ownership: %w", id, err)
	}
	return count > 0, nil
}

// MarkAllRead menandai semua notifikasi belum-baca milik pasien sebagai terbaca,
// mengembalikan jumlah baris yang diperbarui.
func (r *PatientNotificationRepository) MarkAllRead(db *gorm.DB, patientID string) (int64, error) {
	result := db.Model(&entity.PatientNotification{}).
		Where("patient_id = ? AND read_at IS NULL", patientID).
		Update("read_at", time.Now())
	if result.Error != nil {
		return 0, fmt.Errorf("marking all notifications read for patient %s: %w", patientID, result.Error)
	}
	return result.RowsAffected, nil
}
