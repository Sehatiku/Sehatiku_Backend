package main

import (
	"context"
	"errors"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"sehatiku-backend/internal/config"

	"github.com/labstack/echo/v5"
	"go.uber.org/zap"
)

func main() {
	cfg := config.NewViper()
	log := config.NewLogger(cfg)
	db := config.ConnectDB(cfg, log)
	validate := config.NewValidator(cfg)
	redis := config.SetUpRedis(cfg, log)
	app := config.NewEcho(cfg)
	jwt := config.SetUpJWT(cfg, log)
	wa := config.SetUpWhatsApp(cfg, log, db)

	config.BootStrap(&config.BootStrapConfig{
		DB:       db,
		App:      app,
		Log:      log,
		Validate: validate,
		Config:   cfg,
		Redis:    redis,
		JWT:      jwt,
		WhatsApp: wa,
	})

	port := cfg.GetString("APP_PORT")
	if port == "" {
		port = "9000"
	}

	// Graceful shutdown: saat Railway/OS mengirim SIGTERM (atau Ctrl+C), kita PUTUS koneksi WA
	// dengan rapi sebelum proses mati. Tanpa ini, container lama saat redeploy tidak pernah
	// Disconnect() sehingga koneksinya menggantung & berperang reconnect dengan container baru —
	// perang itulah yang akhirnya memicu WhatsApp me-logout device (sesi terhapus, harus scan QR
	// ulang). Disconnect (bukan Logout) tidak menghapus sesi, jadi pairing tetap tersimpan.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	go func() {
		<-ctx.Done()
		log.Info("shutdown signal diterima — memutus koneksi whatsapp")
		wa.Client.Disconnect()
	}()

	// Echo v5 melakukan graceful shutdown HTTP otomatis saat ctx dibatalkan; Start mengembalikan
	// http.ErrServerClosed pada shutdown normal — itu bukan kondisi fatal.
	sc := echo.StartConfig{Address: ":" + port}
	if err := sc.Start(ctx, app); err != nil && !errors.Is(err, http.ErrServerClosed) {
		log.Fatal("server berhenti dengan error", zap.Error(err))
	}

	log.Info("server berhenti dengan rapi")
}
