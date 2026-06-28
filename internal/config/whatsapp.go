package config

import (
	"context"
	"time"

	"sehatiku-backend/internal/gateway/whatsapp"

	"github.com/spf13/viper"
	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/store/sqlstore"
	"go.mau.fi/whatsmeow/types/events"
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

	// Daftarkan event handler SEBELUM Connect supaya tidak ada event awal yang terlewat.
	// Handler ini murni observasi (logging) — tidak mengubah keputusan reconnect. Tujuannya
	// memberi visibilitas kapan & kenapa sesi WA hilang: `StreamReplaced` menandakan koneksi
	// ganda (mis. dua instance saat redeploy), sedangkan `LoggedOut` adalah momen whatsmeow
	// menghapus baris device dari store (unlink HP / HP offline >14 hari / ban).
	registerWhatsAppEventLogging(client, log.Named("whatsmeow-events"))

	if client.Store.ID == nil {
		log.Warn("whatsapp client not paired — run 'go run ./cmd/wa-setup' to scan QR and pair")
	} else {
		// Connect() bersifat asinkron: ia hanya memulai handshake websocket lalu kembali.
		// Tunggu sampai koneksi benar-benar terhubung+login agar log startup jujur dan agar
		// request pertama tidak datang sebelum socket siap. Kalau belum pulih dalam timeout,
		// auto-reconnect tetap akan mencoba di belakang layar.
		if connErr := client.Connect(); connErr != nil {
			log.Error("whatsapp: failed to connect", zap.Error(connErr))
		} else if client.WaitForConnection(30 * time.Second) {
			log.Info("whatsapp client connected")
		} else {
			log.Warn("whatsapp: belum terhubung dalam 30s setelah connect — akan dicoba ulang via auto-reconnect")
		}
	}

	return whatsapp.New(client, log)
}

// registerWhatsAppEventLogging memasang handler yang mencatat event lifecycle koneksi WA.
// Level log dipilih supaya penyebab "device hilang" langsung terlihat di log (mis. Railway):
// StreamReplaced/LoggedOut di-log keras karena itulah jejak koneksi ganda & pencabutan sesi.
func registerWhatsAppEventLogging(client *whatsmeow.Client, log *zap.Logger) {
	client.AddEventHandler(func(evt any) {
		switch e := evt.(type) {
		case *events.Connected:
			log.Info("whatsapp connected")
		case *events.Disconnected:
			log.Warn("whatsapp disconnected (websocket ditutup server)")
		case *events.StreamReplaced:
			log.Warn("whatsapp stream replaced — koneksi digantikan client lain dengan kunci yang sama; " +
				"indikasi koneksi ganda (mis. dua instance saat redeploy / wa-setup dijalankan saat server hidup)")
		case *events.LoggedOut:
			log.Error("whatsapp logged out — sesi device dihapus dari store, perlu scan QR ulang",
				zap.Bool("on_connect", e.OnConnect),
				zap.String("reason", e.Reason.String()),
				zap.String("reason_code", e.Reason.NumberString()),
			)
		case *events.PairSuccess:
			log.Info("whatsapp pairing berhasil", zap.String("jid", e.ID.String()))
		case *events.TemporaryBan:
			log.Error("whatsapp temporary ban", zap.String("ban", e.String()), zap.Duration("expire", e.Expire))
		}
	})
}
