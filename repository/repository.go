package repository

import (
	"context"
	"gorm.io/gorm"
)

type Repository[T any] struct {
	*gorm.DB
}

func (repo Repository[T]) Save(ctx context.Context, db *gorm.DB, entity *T) error {
	return db.WithContext(ctx).Create(entity).Error
}

func (repo Repository[T]) SaveAll(ctx context.Context, db *gorm.DB, entity *[]T) error {
	return db.WithContext(ctx).Create(entity).Error
}

func (repo Repository[T]) Update(ctx context.Context, db *gorm.DB, entity *T) error {
	return db.WithContext(ctx).Save(entity).Error
}

func (repo Repository[T]) Delete(ctx context.Context, db *gorm.DB, entity *T) error {
	return db.WithContext(ctx).Delete(entity).Error
}

func (repo Repository[T]) FindById(ctx context.Context, db *gorm.DB, entity *T, id string) error {
	return db.WithContext(ctx).Where("id = ?", id).Take(entity).Error
}

func (repo Repository[T]) FindAll(ctx context.Context, db *gorm.DB, entity *[]T) error {
	return db.WithContext(ctx).Find(entity).Error
}
