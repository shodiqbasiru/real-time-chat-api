package entity

import "time"

type User struct {
	BaseEntity
	Name        string    `json:"name" gorm:"type:varchar(255)"`
	Email       string    `json:"email" gorm:"unique;type:varchar(100)"`
	Avatar      string    `json:"avatar,omitempty" gorm:"text"`
	PhoneNumber string    `json:"phoneNumber" gorm:"unique;type:varchar(20)"`
	AuthId      string    `json:"authId" gorm:"type:varchar(255);unique"`
	IsOnline    bool      `json:"isOnline" gorm:"default:false"`             
	LastSeen    time.Time `json:"lastSeen" gorm:"default:CURRENT_TIMESTAMP"` 
}
