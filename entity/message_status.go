package entity

import "time"

type MessageStatus struct {
	BaseEntity
	MessageID   string     `json:"messageId" gorm:"type:varchar(255);index"`
	UserID      string     `json:"userId" gorm:"type:varchar(255);index"`
	IsDelivered bool       `json:"isDelivered" gorm:"default:false"` // NEW
	DeliveredAt *time.Time `json:"deliveredAt" gorm:"null"`          // NEW
	IsRead      bool       `json:"isRead" gorm:"default:false"`
	ReadAt      *time.Time `json:"readAt" gorm:"null"`

	Message Messages `json:"-" gorm:"foreignKey:MessageID;references:ID"`
	User    User     `json:"-" gorm:"foreignKey:UserID;references:ID"`
}

func (MessageStatus) TableName() string {
	return "t_message_status"
}
