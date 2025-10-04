package entity

type User struct {
	BaseEntity
	Name        string `json:"name" gorm:"type:varchar(255)"`
	Email       string `json:"email" gorm:"unique;type:varchar(100)"`
	PhoneNumber string `json:"phoneNumber" gorm:"unique;type:varchar(20)"`
	AuthId      string `json:"authId" gorm:"type:varchar(255);unique"`
}
