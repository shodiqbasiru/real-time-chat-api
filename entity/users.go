package entity

type User struct {
	BaseEntity
	Name        string `json:"name" gorm:"type:varchar(255)"`
	Email       string `json:"email" gorm:"unique;type:varchar(100)"`
	Avatar      string `json:"avatar,omitempty" gorm:"text"`
	PhoneNumber string `json:"phoneNumber" gorm:"unique;type:varchar(20)"`
	AuthId      string `json:"authId" gorm:"type:varchar(255);unique"`

	Messages      []Messages        `json:"-" gorm:"foreignKey:SenderId"`
	Participating []ChatParticipant `json:"-" gorm:"foreignKey:UserID"`
}
