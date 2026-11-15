package res

type LoginResponse struct {
	// Token is the short-lived access token (JWT) sent as a Bearer token.
	Token string `json:"token"`
	// TokenType is always "Bearer".
	TokenType string `json:"tokenType"`
	// ExpiresIn is the access token lifetime in seconds.
	ExpiresIn int64 `json:"expiresIn"`
	// RefreshToken is exchanged at POST /auth/refresh for a new pair.
	RefreshToken string `json:"refreshToken"`
	// RefreshExpiresIn is the refresh token lifetime in seconds.
	RefreshExpiresIn int64 `json:"refreshExpiresIn"`
}
