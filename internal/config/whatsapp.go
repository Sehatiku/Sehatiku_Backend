package config

import (
	"context"

	"sehatiku-backend/internal/gateway/whatsapp"

	"github.com/spf13/viper"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/store/sqlstore"
	waLog "go.mau.fi/whatsmeow/util/log"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

// zapWaLog bridges *zap.Logger to the waLog.Logger interface expected by whatsmeow.
type zapWaLog struct {
	log *zap.Logger
}

func (z *zapWaLog) Errorf(msg string, args ...any) { z.log.Sugar().Errorf(msg, args...) }
func (z *zapWaLog) Warnf(msg string, args ...any)  { z.log.Sugar().Warnf(msg, args...) }
func (z *zapWaLog) Infof(msg string, args ...any)  { z.log.Sugar().Infof(msg, args...) }
func (z *zapWaLog) Debugf(msg string, args ...any) { z.log.Sugar().Debugf(msg, args...) }
func (z *zapWaLog) Sub(module string) waLog.Logger {
	return &zapWaLog{log: z.log.Named(module)}
}

// SetUpWhatsApp initialises the whatsmeow client using the same PostgreSQL connection
// that GORM already holds. Session credentials are stored in whatsmeow's own tables
// (created automatically on first run). If no pairing exists, the client starts in an
// unauthenticated state and WA messages will silently fail until cmd/wa-setup is run.
func SetUpWhatsApp(_ *viper.Viper, log *zap.Logger, db *gorm.DB) *whatsapp.WhatsAppGateway {
	ctx := context.Background()
	waLogger := &zapWaLog{log: log.Named("whatsmeow")}

	sqlDB, err := db.DB()
	if err != nil {
		log.Fatal("whatsapp: failed to get sql.DB from gorm", zap.Error(err))
	}

	container := sqlstore.NewWithDB(sqlDB, "postgres", waLogger)
	if err := container.Upgrade(ctx); err != nil {
		log.Fatal("whatsapp: failed to upgrade whatsmeow schema", zap.Error(err))
	}

	deviceStore, err := container.GetFirstDevice(ctx)
	if err != nil {
		log.Fatal("whatsapp: failed to get device store", zap.Error(err))
	}

	client := whatsmeow.NewClient(deviceStore, waLogger)
	client.EnableAutoReconnect = true

	if client.Store.ID == nil {
		log.Warn("whatsapp client not paired — run 'go run ./cmd/wa-setup' to scan QR and pair")
	} else {
		if connErr := client.Connect(); connErr != nil {
			log.Error("whatsapp: failed to connect", zap.Error(connErr))
		} else {
			log.Info("whatsapp client connected")
		}
	}

	return whatsapp.New(client, log)
}
