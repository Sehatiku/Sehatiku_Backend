package config

import (
	"github.com/joho/godotenv"
	"github.com/spf13/viper"
)

func NewViper() *viper.Viper {
	godotenv.Load()

	cfg := viper.New()
	cfg.AutomaticEnv()

	return cfg
}
