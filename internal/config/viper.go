package config

import (
	"os"
	"path/filepath"

	"github.com/joho/godotenv"
	"github.com/spf13/viper"
)

func NewViper() *viper.Viper {
	loadDotEnv()

	cfg := viper.New()
	cfg.AutomaticEnv()

	return cfg
}

func loadDotEnv() {
	dir, err := os.Getwd()
	if err != nil {
		_ = godotenv.Load()
		return
	}

	for {
		envPath := filepath.Join(dir, ".env")
		if _, statErr := os.Stat(envPath); statErr == nil {
			_ = godotenv.Load(envPath)
			return
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	_ = godotenv.Load()
}
