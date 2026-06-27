package main

import (
	"sehatiku-backend/internal/config"
	"sehatiku-backend/internal/entity"

	"go.uber.org/zap"
)

func main() {
	cfg := config.NewViper()
	logger := config.NewLogger(cfg)
	db := config.ConnectDB(cfg, logger)

	if err := db.AutoMigrate(
		&entity.User{},
		&entity.Children{},
	); err != nil {
		logger.Fatal("migration failed", zap.Error(err))
	}

	logger.Info("migrations applied successfully")
}
