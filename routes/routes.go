package routes

import (
	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"
	"real-time-chat-app/handler"
	"real-time-chat-app/middleware"
)

type ConfigRoute struct {
	*fiber.App
	*middleware.Middleware
	*handler.AuthHandler
	*handler.UserHandler
	*handler.ChatHandler
}

func (rc *ConfigRoute) GetRoute() {
	rc.GetPublicRoute()
	rc.GetProtectedRoute()
}

func (rc *ConfigRoute) GetPublicRoute() {
	app := rc.App.Group("/api/v1")
	app.Post("/auth/register", rc.AuthHandler.RegisterUser)
	app.Post("/auth/login", rc.AuthHandler.LoginUser)
}

func (rc *ConfigRoute) GetProtectedRoute() {
	app := rc.App.Group("/api/v1")
	app.Use(rc.Middleware.JWTProtected)

	app.Get("/auth/me", rc.UserHandler.GetUserByToken)

	app.Get("/users", rc.UserHandler.GetAllUsers)

	app.Get("/chats/:chatId/messages", rc.ChatHandler.GetMessagesByID)

	app.Get("/chats", rc.ChatHandler.GetAllChat)
}

func (rc *ConfigRoute) GetWebSocketRoute(wsHandler *handler.WebSocketHandler) {
	rc.App.Use("/ws", func(c *fiber.Ctx) error {
		if websocket.IsWebSocketUpgrade(c) {
			c.Locals("allowed", true)
			return c.Next()
		}
		return fiber.ErrUpgradeRequired
	})

	rc.App.Get("/ws", websocket.New(wsHandler.HandleWebSocket))
}
