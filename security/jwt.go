package security

import (
	"github.com/gofiber/fiber/v2/log"
	"github.com/golang-jwt/jwt/v5"
	"real-time-chat-app/config/common"
	"real-time-chat-app/entity"
	"time"
)

type JWT struct {
	config *common.Config
}

func NewJWT(config *common.Config) *JWT {
	return &JWT{config: config}
}

func (j *JWT) GenerateToken(user *entity.User) (string, error) {
	secretKey := j.config.GetJwtConfig()

	claims := jwt.MapClaims{
		"user_id": user.ID,
		"aud":     "real-time-chat-app",
		"iss":     "real-time-chat-app",
		"iat":     time.Now().Unix(),
		"exp":     time.Now().Add(time.Hour * 1).Unix(),
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS512, claims)
	return token.SignedString(secretKey)
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

	log.Infof("User ID From JWT: %s", userID)

	if !ok {
		return "", jwt.ErrInvalidKey
	}

	return userID, nil
}
