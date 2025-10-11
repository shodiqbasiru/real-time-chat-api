package entity

type UserChat struct {
	BaseEntity
	UserID string `json:"userId" gorm:"type:varchar(255)"`
	ChatID string `json:"chatId" gorm:"type:varchar(255)"`

	User User `json:"-" gorm:"foreignKey:UserID;references:ID"`
	Chat Chat `json:"-" gorm:"foreignKey:ChatID;references:ID"`
}
