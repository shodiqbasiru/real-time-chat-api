package security

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"real-time-chat-app/config/common"
	"real-time-chat-app/entity"
)

type JWT struct {
	config *common.Config
}

func NewJWT(config *common.Config) *JWT {
	return &JWT{config: config}
}

func (j *JWT) GenerateToken(user *entity.User) (string, error) {
	return j.GenerateAccessToken(user.ID)
}

// GenerateAccessToken signs a short-lived access token for the user.
func (j *JWT) GenerateAccessToken(userID string) (string, error) {
	secretKey := j.config.GetJwtConfig()
	now := time.Now()

	claims := jwt.MapClaims{
		"user_id": userID,
		"aud":     "real-time-chat-app",
		"iss":     "real-time-chat-app",
		"iat":     now.Unix(),
		"exp":     now.Add(j.AccessTokenTTL()).Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS512, claims)
	return token.SignedString(secretKey)
}

func (j *JWT) AccessTokenTTL() time.Duration {
	return j.config.GetAccessTokenTTL()
}

func (j *JWT) RefreshTokenTTL() time.Duration {
	return j.config.GetRefreshTokenTTL()
}

// NewRefreshToken returns a random opaque refresh token and the hash that
// is stored in the database.
func NewRefreshToken() (token string, hash string, err error) {
	buf := make([]byte, 32)
	if _, err := rand.Read(buf); err != nil {
		return "", "", err
	}
	token = base64.RawURLEncoding.EncodeToString(buf)
	return token, HashRefreshToken(token), nil
}

// HashRefreshToken hashes a refresh token for lookup. The token has 256
// bits of entropy, so a plain SHA-256 is enough.
func HashRefreshToken(token string) string {
	sum := sha256.Sum256([]byte(token))
	return hex.EncodeToString(sum[:])
}

func (j *JWT) VerifyJwtToken(token string) (jwt.MapClaims, error) {
	secretKey := j.config.GetJwtConfig()

	tokenParse, err := jwt.Parse(token, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, jwt.ErrSignatureInvalid
		}
		return secretKey, nil
	})

	if err != nil {
		return nil, err
	}

	if claims, ok := tokenParse.Claims.(jwt.MapClaims); ok && tokenParse.Valid {
		return claims, nil
	}

	return nil, err
}

func (j *JWT) GetUserIdFromToken(token string) (string, error) {
	claims, err := j.VerifyJwtToken(token)
	if err != nil {
		return "", err
	}

	userID, ok := claims["user_id"].(string)

	if !ok {
		return "", jwt.ErrInvalidKey
	}

	return userID, nil
}
