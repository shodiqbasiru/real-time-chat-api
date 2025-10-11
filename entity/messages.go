package entity

type Messages struct {
	BaseEntity
	Content  string `json:"content" gorm:"type:TEXT"`
	ChatId   string `json:"chatId" gorm:"foreignKey"`
	SenderId string `json:"senderId" gorm:"foreignKey"`

	Chat   Chat `json:"-" gorm:"foreignKey:ChatId;references:ID"`
	Sender User `json:"-" gorm:"foreignKey:SenderId;references:ID"`
}
