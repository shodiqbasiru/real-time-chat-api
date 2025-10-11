package entity

import "real-time-chat-app/enum"

type Chat struct {
	BaseEntity
	ChatType  enum.ChatType `json:"chatType" gorm:"type:varchar(7)"`
	GroupName string        `json:"groupName" gorm:"type:varchar(50);null"`

	Participants []ChatParticipant `json:"participants" gorm:"foreignKey:ChatID;constraint:OnDelete:CASCADE;"`
	Messages     []Messages        `json:"messages" gorm:"foreignKey:ChatId;constraint:OnDelete:CASCADE;"`
}

type ChatParticipant struct {
	ID     string `gorm:"primaryKey;type:varchar(255);default:gen_random_uuid()"`
	ChatID string `gorm:"type:varchar(255);not null"`
	UserID string `gorm:"type:varchar(255);not null"`

	Chat Chat `gorm:"foreignKey:ChatID;references:ID;constraint:OnDelete:CASCADE;"`
	User User `gorm:"foreignKey:UserID;references:ID;constraint:OnDelete:CASCADE;"`
}
