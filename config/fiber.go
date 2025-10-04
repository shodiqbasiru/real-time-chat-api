package config

import (
	"github.com/gofiber/fiber/v2"
	"real-time-chat-app/config/common"
)

func NewFiber(cfg *common.Config) *fiber.App {
	appName := cfg.GetAppConfig()
	return fiber.New(fiber.Config{
		Prefork:       false,
		CaseSensitive: true,
		StrictRouting: true,
		AppName:       appName,
	})
}
