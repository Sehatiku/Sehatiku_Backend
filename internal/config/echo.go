package config

import (
	"net/http"

	"github.com/labstack/echo/v5"
	echomiddleware "github.com/labstack/echo/v5/middleware"
	"github.com/spf13/viper"
)

func NewEcho(config *viper.Viper) *echo.Echo {
	app := echo.New()

	app.Use(echomiddleware.CORSWithConfig(echomiddleware.CORSConfig{
		AllowOrigins: []string{
			"https://sehatiku.vercel.app",
			"http://localhost:5174",
		},
		AllowMethods: []string{
			http.MethodGet, http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete, http.MethodOptions,
		},
		AllowHeaders: []string{
			echo.HeaderOrigin, echo.HeaderContentType, echo.HeaderAccept, echo.HeaderAuthorization,
		},
		AllowCredentials: true,
	}))

	return app
}
