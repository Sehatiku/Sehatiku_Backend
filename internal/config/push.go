package config

import (
	"context"

	firebase "firebase.google.com/go/v4"
	pushgw "sehatiku-backend/internal/gateway/push"

	"github.com/spf13/viper"
	"go.uber.org/zap"
	"google.golang.org/api/option"
)

// SetUpPush menginisialisasi Firebase Admin SDK untuk push notification (FCM) Patient App.
// Kredensial diprioritaskan dari FIREBASE_CREDENTIALS_JSON (isi service-account JSON langsung
// sebagai env var — dipakai di platform tanpa filesystem persisten seperti Railway, supaya
// tidak perlu commit file credential ke repo). Bila kosong, fallback ke FIREBASE_CREDENTIALS_FILE
// (path file service-account JSON, dipakai untuk dev lokal). Keduanya kosong atau gagal
// inisialisasi -> Client nil, fitur push no-op (server tetap start normal) — sama falsafah
// graceful-degradation dengan SetUpWhatsApp saat device belum dipasangkan.
func SetUpPush(cfg *viper.Viper, log *zap.Logger) *pushgw.PushGateway {
	ctx := context.Background()

	var opt option.ClientOption
	switch {
	case cfg.GetString("FIREBASE_CREDENTIALS_JSON") != "":
		opt = option.WithCredentialsJSON([]byte(cfg.GetString("FIREBASE_CREDENTIALS_JSON")))
	case cfg.GetString("FIREBASE_CREDENTIALS_FILE") != "":
		opt = option.WithCredentialsFile(cfg.GetString("FIREBASE_CREDENTIALS_FILE"))
	default:
		log.Warn("push: FIREBASE_CREDENTIALS_JSON/FIREBASE_CREDENTIALS_FILE kosong — push notification dinonaktifkan")
		return &pushgw.PushGateway{Client: nil, Log: log}
	}

	app, err := firebase.NewApp(ctx, nil, opt)
	if err != nil {
		log.Error("push: gagal inisialisasi firebase app", zap.Error(err))
		return &pushgw.PushGateway{Client: nil, Log: log}
	}

	client, err := app.Messaging(context.Background())
	if err != nil {
		log.Error("push: gagal inisialisasi firebase messaging client", zap.Error(err))
		return &pushgw.PushGateway{Client: nil, Log: log}
	}

	log.Info("push: firebase messaging client siap")
	return &pushgw.PushGateway{Client: client, Log: log}
}
