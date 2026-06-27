package config

import (
	"time"

	"github.com/spf13/viper"
	"go.uber.org/zap"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func ConnectDB(viper *viper.Viper, log *zap.Logger) *gorm.DB {
	dbURL := viper.GetString("DATABASE_URL")

	if dbURL == "" {
		log.Fatal("DATABASE_URL is  required")
	}

	db, err := gorm.Open(postgres.Open(dbURL+"?sslmode=require"), &gorm.Config{})

	if err != nil {
		log.Fatal("failed to connect database", zap.Error(err))
	}

	sqlDB, err := db.DB()
	if err != nil {
		log.Fatal("failed to get db instance", zap.Error(err))
	}

	sqlDB.SetMaxIdleConns(5)
	sqlDB.SetMaxOpenConns(10)
	sqlDB.SetConnMaxLifetime(5 * time.Minute)

	return db
}
