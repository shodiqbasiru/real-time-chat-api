package entity

import "real-time-chat-app/enum"

type Messages struct {
	BaseEntity
	Content  string             `json:"content" gorm:"type:TEXT"`
	ChatId   string             `json:"chatId" gorm:"foreignKey"`
	SenderId string             `json:"senderId" gorm:"foreignKey"`
	Status   enum.MessageStatus ` json:"status" gorm:"type:varchar(20);default:'sent'"`

	Chat   Chat `json:"-" gorm:"foreignKey:ChatId;references:ID"`
	Sender User `json:"-" gorm:"foreignKey:SenderId;references:ID"`
}
