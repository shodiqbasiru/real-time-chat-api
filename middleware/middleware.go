package middleware

import (
	jwtware "github.com/gofiber/contrib/jwt"
	"github.com/gofiber/fiber/v2"
	"real-time-chat-app/config/common"
	"real-time-chat-app/config/logger"
	"real-time-chat-app/dto/res"
	"real-time-chat-app/security"
)

type Middleware struct {
	*common.Config
	*security.JWT
	Log *logger.AppLogger
}

func NewMiddleware(config *common.Config, logger *logger.AppLogger) *Middleware {
	return &Middleware{Config: config, Log: logger}
}

func (middleware *Middleware) JWTProtected(c *fiber.Ctx) error {
	secretKey := middleware.GetJwtConfig()

	return jwtware.New(jwtware.Config{
		SigningKey: jwtware.SigningKey{Key: secretKey},
		ContextKey: "jwt",
		ErrorHandler: func(ctx *fiber.Ctx, err error) error {
			middleware.Log.Http.Error.Err(err).Msg("Failed to validate JWT")
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
		middleware.Log.Http.Error.Err(err).Msg("Failed to extract user ID from token")
		return c.Status(fiber.StatusUnauthorized).JSON(res.ErrorResponse{
			Status:     fiber.ErrUnauthorized.Message,
			StatusCode: fiber.StatusUnauthorized,
			Error:      "Failed to extract user ID from token",
		})
	}

	middleware.Log.Http.Info.Info().Msgf("User ID From Middleware: %v", userID)
	c.Locals("user_id", userID)
	return c.Next()
}
