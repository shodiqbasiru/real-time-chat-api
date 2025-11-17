package entity

import "time"

type MessageStatus struct {
	BaseEntity
	IsRead    bool      `json:"isRead" gorm:"default:false"`
	ReadAt    time.Time `json:"readAt" gorm:"null"`
	MessageID string    `json:"messageID" gorm:"foreignKey"`
	UserID    string    `json:"userID" gorm:"foreignKey"`
}
