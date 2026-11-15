package common

import (
	"time"

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
	config.SetDefault("JWT_ACCESS_TTL", "15m")
	config.SetDefault("JWT_REFRESH_TTL", "168h")

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

// GetAccessTokenTTL is how long an access token (JWT) is valid.
func (c *Config) GetAccessTokenTTL() time.Duration {
	if ttl := c.Viper.GetDuration("JWT_ACCESS_TTL"); ttl > 0 {
		return ttl
	}
	return 15 * time.Minute
}

// GetRefreshTokenTTL is how long a refresh token is valid. Each refresh
// issues a new one, so a session stays alive while it is used at least once
// per TTL.
func (c *Config) GetRefreshTokenTTL() time.Duration {
	if ttl := c.Viper.GetDuration("JWT_REFRESH_TTL"); ttl > 0 {
		return ttl
	}
	return 7 * 24 * time.Hour
}
