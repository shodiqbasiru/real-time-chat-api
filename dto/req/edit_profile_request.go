package req

type EditProfileRequest struct {
	Name        string `json:"name" validate:"required,min=2,max=50"`
	PhoneNumber string `json:"phoneNumber" validate:"required,min=8,max=20"`
}
