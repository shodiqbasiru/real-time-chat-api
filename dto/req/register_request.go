package req

type RegisterRequest struct {
	Username    string `json:"username" validate:"required,min=3"`
	PhoneNumber string `json:"phoneNumber" validate:"required,min=8"`
	Email       string `json:"email" validate:"required,email"`
	Password    string `json:"password" validate:"required,min=6"`
}
