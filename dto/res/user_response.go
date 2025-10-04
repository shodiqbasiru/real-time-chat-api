package res

type UserResponse struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Email       string `json:"email"`
	PhoneNumber string `json:"phoneNumber"`
	CreatedAt   string `json:"createdAt"`
}
