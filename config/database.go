package config

import (
	"fmt"
	"github.com/sirupsen/logrus"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
	"real-time-chat-app/config/common"
	"real-time-chat-app/entity"
	"time"
)

type DBConfig struct {
	*gorm.DB
	*logrus.Logger
}

func NewDB(config *common.Config, log *logrus.Logger) *DBConfig {
	db := initDatabase(config, log)
	return &DBConfig{DB: db, Logger: log}
}

func (db *DBConfig) GetDB() *gorm.DB {
	return db.DB
}

func initDatabase(cfg *common.Config, log *logrus.Logger) *gorm.DB {
	dbHost, dbUser, dbPassword, dbName, dbPort := cfg.GetDatabaseConfig()
	dsn := fmt.Sprintf(
		"host=%s user=%s password=%s dbname=%s port=%s sslmode=disable",
		dbHost, dbUser, dbPassword, dbName, dbPort,
	)
	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		NamingStrategy: schema.NamingStrategy{
			TablePrefix:   "t_",
			SingularTable: true,
		},
	})
	if err != nil {
		log.WithError(err).Errorf("falied to connect to database : %v", err)
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
	var userChat entity.UserChat
	if err := db.AutoMigrate(&auth, &user, &chat, &chatParticipant, &messages, &userChat); err != nil {
		panic("failed run migration")
	}

	conn.SetMaxIdleConns(10)
	conn.SetMaxOpenConns(100)
	conn.SetConnMaxLifetime(time.Second * time.Duration(300))

	return db
}
