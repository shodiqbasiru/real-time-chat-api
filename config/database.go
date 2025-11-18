package config

import (
	"fmt"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
	"real-time-chat-app/config/common"
	"real-time-chat-app/config/logger"
	"real-time-chat-app/entity"
	"time"
)

type DBConfig struct {
	*gorm.DB
	*logger.AppLogger
}

func NewDB(config *common.Config, log *logger.AppLogger) *DBConfig {
	db := initDatabase(config, log)
	return &DBConfig{DB: db, AppLogger: log}
}

func (db *DBConfig) GetDB() *gorm.DB {
	return db.DB
}

func initDatabase(cfg *common.Config, log *logger.AppLogger) *gorm.DB {
	dbHost, dbUser, dbPassword, dbName, dbPort := cfg.GetDatabaseConfig()
	dsn := fmt.Sprintf(
		"host=%s user=%s password=%s dbname=%s port=%s sslmode=disable TimeZone=Asia/Jakarta",
		dbHost, dbUser, dbPassword, dbName, dbPort,
	)
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{
			TablePrefix:   "t_",
			SingularTable: true,
		},
	})
	if err != nil {
		log.Http.Error.Error().Err(err).Msg("failed to connect to database : %v")
	}

	fmt.Println("Connection Opened to Database")
	conn, err := db.DB()
	if err != nil {
		panic("failed to connect database")
	}

	var auth entity.Account
	var user entity.User
	var chat entity.Chat
	var chatParticipant entity.ChatParticipant
	var messages entity.Messages
	var messageStatus entity.MessageStatus
	if err := db.AutoMigrate(&auth, &user, &chat, &chatParticipant, &messages, &messageStatus); err != nil {
		panic("failed run migration")
	}

	conn.SetMaxIdleConns(10)
	conn.SetMaxOpenConns(100)
	conn.SetConnMaxLifetime(time.Second * time.Duration(300))
	return db
}
