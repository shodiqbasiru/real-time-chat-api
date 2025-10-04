package entity

import (
	"github.com/google/uuid"
	"gorm.io/gorm"
	"time"
)

type BaseEntity struct {
	ID        string         `json:"id" gorm:"primaryKey;type:varchar(255)"`
	CreatedAt time.Time      `json:"createdAt" gorm:"autoCreateTime"`
	UpdatedAt time.Time      `json:"updatedAt" gorm:"autoUpdateTime"`
	DeletedAt gorm.DeletedAt `json:"deletedAt,omitempty" gorm:"index"`
}

func (base *BaseEntity) BeforeCreate(tx *gorm.DB) error {
	if base.ID == "" {
		base.ID = uuid.New().String()
	}
	return nil
}
