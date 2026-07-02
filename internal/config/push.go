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
// Kredensial dibaca dari FIREBASE_CREDENTIALS_FILE (path file service-account JSON). Kosong
// atau gagal inisialisasi -> Client nil, fitur push no-op (server tetap start normal) — sama
// falsafah graceful-degradation dengan SetUpWhatsApp saat device belum dipasangkan.
func SetUpPush(cfg *viper.Viper, log *zap.Logger) *pushgw.PushGateway {
	credFile := cfg.GetString("FIREBASE_CREDENTIALS_FILE")
	if credFile == "" {
		log.Warn("push: FIREBASE_CREDENTIALS_FILE kosong — push notification dinonaktifkan")
		return &pushgw.PushGateway{Client: nil, Log: log}
	}

	app, err := firebase.NewApp(context.Background(), nil, option.WithCredentialsFile(credFile))
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
