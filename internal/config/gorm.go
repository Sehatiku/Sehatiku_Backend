package config

import (
	"strings"
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

	if !strings.Contains(dbURL, "sslmode=") {
		if strings.Contains(dbURL, "?") {
			dbURL += "&sslmode=require"
		} else {
			dbURL += "?sslmode=require"
		}
	}

	// Session pooler Supabase (port 5432) mendukung prepared statements, jadi kita biarkan
	// pgx pakai protokol extended (default) — lebih cepat dan membuat codec tipe (mis. jsonb)
	// bekerja normal. JANGAN aktifkan PreferSimpleProtocol kecuali pindah ke transaction
	// pooler (6543), yang tidak mendukung prepared statements.
	db, err := gorm.Open(postgres.New(postgres.Config{
		DSN: dbURL,
	}), &gorm.Config{})

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
