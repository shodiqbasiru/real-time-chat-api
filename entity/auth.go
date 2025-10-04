package entity

type Account struct {
	BaseEntity
	UserName string `json:"userName" gorm:"unique;type:varchar(50)"`
	Password string `json:"password" gorm:"type:varchar(255)"`
	User     User   `gorm:"foreignKey:AuthId;references:ID"`
}
