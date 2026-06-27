package routing

import (
	"sehatiku-backend/internal/helper"

	"github.com/labstack/echo/v5"
)

type RouteConfig struct {
	App       *echo.Echo
	JWTHelper *helper.JWTHelper
}

func (r *RouteConfig) SetUp() {
	r.SetupGuestRoute()
}
func (r *RouteConfig) SetupGuestRoute() {
}
