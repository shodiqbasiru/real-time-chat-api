package req

type EditProfileRequest struct {
	Name        string `json:"name" validate:"required;min=2"`
	PhoneNumber string `json:"phoneNumber" validate:"required;=min3;max=15"`
}
