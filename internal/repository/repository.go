package repository

import (
	"gorm.io/gorm"
)

type Repository[T any] struct {
	DB *gorm.DB
}

func (r *Repository[T]) Create(db *gorm.DB, entity *T) error {
	return db.Create(entity).Error
}
func (r *Repository[T]) Update(db *gorm.DB, entity *T) error {
	return db.Save(entity).Error
}
func (r *Repository[T]) Delete(db *gorm.DB, entity *T) error {
	return db.Delete(entity).Error
}
func (r *Repository[T]) FindByCondition(db *gorm.DB, condition string, args ...interface{}) (*T, error) {
	var data T

	if err := db.Where(condition, args...).Find(&data).Error; err != nil {
		return nil, err
	}
	return &data, nil
}
func (r *Repository[T]) Find(db *gorm.DB) (*[]T, error) {
	var data []T

	if err := db.Find(&data).Error; err != nil {
		return nil, err
	}
	return &data, nil
}
