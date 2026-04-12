package config

import (
	"real-time-chat-app/config/common"
	"real-time-chat-app/config/logger"
	"real-time-chat-app/handler"
	"real-time-chat-app/middleware"
	"real-time-chat-app/repository"
	"real-time-chat-app/routes"
	"real-time-chat-app/security"
	"real-time-chat-app/usecase"

	"github.com/go-playground/validator/v10"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
)

type AppConfig struct {
	*fiber.App
	*validator.Validate
	*logger.AppLogger
	*DBConfig
	*security.JWT
	*middleware.Middleware
}

func RunServer() {
	newConfig := common.NewViper()
	app := NewFiber(newConfig)
	log := logger.NewLogger()
	newDB := NewDB(newConfig, log)
	newValidator := NewValidator()
	newJWT := security.NewJWT(newConfig)
	newMiddleware := middleware.NewMiddleware(newConfig, log)

	// middleware CORS
	app.Use(cors.New(cors.Config{
		AllowOrigins: "http://localhost:3000",
		AllowMethods: "GET,POST,PUT,DELETE,OPTIONS",
		AllowHeaders: "Origin, Content-Type, Accept, Authorization",
		//AllowCredentials: true, // FIX: Enable credentials untuk WebSocket
	}))

	App(&AppConfig{
		App:        app,
		Validate:   newValidator,
		AppLogger:  log,
		DBConfig:   newDB,
		JWT:        newJWT,
		Middleware: newMiddleware,
	})

	if err := app.Listen(":7720"); err != nil {
		log.Http.Error.Error().Err(err).Msg("Failed to start server")
	}
}

func App(aC *AppConfig) {
	newAuthRepository := repository.NewAuthRepository()
	newUserRepository := repository.NewUserRepository()
	newChatRepository := repository.NewChatRepository()

	newAuthUsecase := usecase.NewAuthUsecase(newAuthRepository, aC.Validate, aC.GetDB(), aC.AppLogger, aC.JWT)
	newUserUseCase := usecase.NewUserUsecase(newUserRepository, aC.Validate, aC.GetDB(), aC.AppLogger, aC.JWT)
	newMessageUsecase := usecase.NewMessageUsecase(aC.DB, aC.AppLogger)
	newChatUsecase := usecase.NewChatUsecase(newChatRepository, newMessageUsecase, newUserUseCase, aC.AppLogger, aC.GetDB(), aC.JWT)

	newAuthHandler := handler.NewAuthHandler(newAuthUsecase, aC.AppLogger)
	newUserHandler := handler.NewUserHandler(newUserUseCase, aC.AppLogger)
	newChatHandler := handler.NewChatHandler(newChatUsecase, newMessageUsecase, aC.AppLogger, aC.JWT)

	wsHandler := handler.NewWebSocketHandler(aC.GetDB(), aC.AppLogger, newChatUsecase, newMessageUsecase)

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
