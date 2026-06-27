package config

import (
	"sehatiku-backend/internal/helper"

	"github.com/spf13/viper"
	"go.uber.org/zap"
)

func SetUpJWT(viper *viper.Viper, log *zap.Logger) *helper.JWTHelper {
	secret := viper.GetString("JWT_SECRET")

	return &helper.JWTHelper{
		Secret: secret,
		Log:    log,
	}
}
