package config

import (
	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/sirupsen/logrus"
	"real-time-chat-app/config/common"
	"real-time-chat-app/handler"
	"real-time-chat-app/middleware"
	"real-time-chat-app/repository"
	"real-time-chat-app/routes"
	"real-time-chat-app/security"
	"real-time-chat-app/usecase"
)

type AppConfig struct {
	*fiber.App
	*validator.Validate
	*logrus.Logger
	*DBConfig
	*security.JWT
	*middleware.Middleware
}

func RunServer() {
	newConfig := common.NewViper()
	app := NewFiber(newConfig)
	log := NewLogger()
	newDB := NewDB(newConfig, log)
	newValidator := NewValidator()
	newJWT := security.NewJWT(newConfig)
	newMiddleware := middleware.NewMiddleware(newConfig, log)

	// middleware CORS
	app.Use(cors.New(cors.Config{
		AllowOrigins: "http://localhost:8080",
		AllowMethods: "GET,POST,PUT,DELETE,OPTIONS",
		AllowHeaders: "Origin, Content-Type, Accept, Authorization",
		//AllowCredentials: true, // FIX: Enable credentials untuk WebSocket
	}))

	App(&AppConfig{
		App:        app,
		Validate:   newValidator,
		Logger:     log,
		DBConfig:   newDB,
		JWT:        newJWT,
		Middleware: newMiddleware,
	})

	if err := app.Listen(":7720"); err != nil {
		log.WithError(err).Errorf("Failed to start server: %v", err)
	}
}

func App(aC *AppConfig) {
	newAuthRepository := repository.NewAuthRepository()
	newUserRepository := repository.NewUserRepository()
	newChatRepository := repository.NewChatRepository()

	newAuthUsecase := usecase.NewAuthUsecase(newAuthRepository, aC.Validate, aC.GetDB(), aC.Logger, aC.JWT)
	newAuthCase := usecase.NewUserUsecase(newUserRepository, aC.Validate, aC.GetDB(), aC.Logger, aC.JWT)
	newChatUsecase := usecase.NewChatUsecase(newChatRepository, aC.Logger, aC.GetDB(), aC.JWT)
	newMessageUsecase := usecase.NewMessageUsecase(aC.DB, newChatUsecase)

	newAuthHandler := handler.NewAuthHandler(newAuthUsecase, aC.Logger)
	newUserHandler := handler.NewUserHandler(newAuthCase, aC.Logger)
	newChatHandler := handler.NewChatHandler(newChatUsecase, newMessageUsecase, aC.Logger, aC.JWT)

	wsHandler := handler.NewWebSocketHandler(aC.GetDB(), aC.Logger, newChatUsecase, newMessageUsecase)

	route := routes.ConfigRoute{
		App:         aC.App,
		Middleware:  aC.Middleware,
		AuthHandler: newAuthHandler,
		UserHandler: newUserHandler,
		ChatHandler: newChatHandler,
	}
	route.GetRoute()
	route.GetWebSocketRoute(wsHandler)
}
