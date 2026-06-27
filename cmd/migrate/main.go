package main

import (
	"sehatiku-backend/internal/config"
	"sehatiku-backend/internal/entity"

	"go.uber.org/zap"
)

// cmd/migrate hanya digunakan untuk keperluan development.
// Migrasi production dilakukan via SQL files di db/migration/ (golang-migrate style).
func main() {
	cfg := config.NewViper()
	logger := config.NewLogger(cfg)
	db := config.ConnectDB(cfg, logger)

	if err := db.AutoMigrate(
		&entity.Faskes{},
		&entity.Nakes{},
		&entity.Patient{},
	); err != nil {
		logger.Fatal("migration failed", zap.Error(err))
	}

	logger.Info("migrations applied successfully")
}
