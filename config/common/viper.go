package common

import (
	"github.com/gofiber/fiber/v2/log"
	"github.com/spf13/viper"
)

type Config struct {
	Viper *viper.Viper
}

func NewViper() *Config {
	config := viper.New()
	config.SetConfigFile(".env")
	config.AddConfigPath("../")
	config.AutomaticEnv()

	log.Trace("Checking file .env ....")
	if err := config.ReadInConfig(); err != nil {
		panic("failed read config")
	}
	return &Config{Viper: config}
}

func (c *Config) GetAppConfig() (appName string) {
	return c.Viper.GetString("APP_NAME")
}

func (c *Config) GetDatabaseConfig() (dbHost, dbUser, dbPassword, dbName, dbPort string) {
	dbHost = c.Viper.GetString("DB_HOSTNAME")
	dbUser = c.Viper.GetString("DB_USER")
	dbPassword = c.Viper.GetString("DB_PASSWORD")
	dbName = c.Viper.GetString("DB_NAME")
	dbPort = c.Viper.GetString("DB_PORT")

	return dbHost, dbUser, dbPassword, dbName, dbPort
}

func (c *Config) GetJwtConfig() []byte {
	jwtSecret := c.Viper.GetString("JWT_SECRET")
	return []byte(jwtSecret)
}
