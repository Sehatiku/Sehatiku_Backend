package config

import (
	"github.com/labstack/echo/v5"
	"github.com/spf13/viper"
)

func NewEcho(config *viper.Viper) *echo.Echo {
	app := echo.New()
	return app
}
