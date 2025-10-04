package routes

import (
	"github.com/gofiber/fiber/v2"
	"real-time-chat-app/handler"
	"real-time-chat-app/middleware"
)

type ConfigRoute struct {
	*fiber.App
	*middleware.Middleware
	*handler.AuthHandler
	*handler.UserHandler
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
}
