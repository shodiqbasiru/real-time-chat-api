package middleware

import (
	jwtware "github.com/gofiber/contrib/jwt"
	"github.com/gofiber/fiber/v2"
	"github.com/sirupsen/logrus"
	"real-time-chat-app/config/common"
	"real-time-chat-app/dto/res"
	"real-time-chat-app/security"
)

type Middleware struct {
	*common.Config
	*security.JWT
	Log *logrus.Logger
}

func NewMiddleware(config *common.Config, logger *logrus.Logger) *Middleware {
	return &Middleware{Config: config, Log: logger}
}

func (middleware *Middleware) JWTProtected(c *fiber.Ctx) error {
	secretKey := middleware.GetJwtConfig()

	return jwtware.New(jwtware.Config{
		SigningKey: jwtware.SigningKey{Key: secretKey},
		ContextKey: "jwt",
		ErrorHandler: func(ctx *fiber.Ctx, err error) error {
			middleware.Log.WithError(err).Error("Failed to validate JWT")
			return c.Status(fiber.StatusUnauthorized).JSON(res.ErrorResponse{
				Status:     fiber.ErrUnauthorized.Message,
				StatusCode: fiber.StatusUnauthorized,
				Error:      "Token is not valid",
			})
		},
	})(c)
}

func (middleware *Middleware) ExtractUserID(c *fiber.Ctx) error {
	token := c.Get("Authorization")[7:]
	userID, err := middleware.JWT.GetUserIdFromToken(token)

	if err != nil {
		middleware.Log.WithError(err).Error("Failed to extract user ID from token")
		return c.Status(fiber.StatusUnauthorized).JSON(res.ErrorResponse{
			Status:     fiber.ErrUnauthorized.Message,
			StatusCode: fiber.StatusUnauthorized,
			Error:      "Failed to extract user ID from token",
		})
	}

	middleware.Log.Info("User ID From Middleware: ", userID)
	c.Locals("user_id", userID)
	return c.Next()
}
