package middleware

import (
	"net/http"
	"sehatiku-backend/internal/constants"
	"sehatiku-backend/internal/helper"
	"sehatiku-backend/internal/model"
	"strings"

	"github.com/labstack/echo/v5"
)

func NakesAuth(jwt *helper.JWTHelper) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			raw := extractBearerToken(c)
			if raw == "" {
				return c.JSON(http.StatusUnauthorized, model.WebResponse[any]{
					Message: constants.MsgUnauthorized,
					Errors:  "authorization header diperlukan",
				})
			}

			claims, err := jwt.ValidateNakesToken(raw)
			if err != nil {
				return c.JSON(http.StatusUnauthorized, model.WebResponse[any]{
					Message: constants.MsgUnauthorized,
					Errors:  "token tidak valid atau sudah kadaluarsa",
				})
			}

			c.Set("nakes_auth", claims)
			c.Set("auth_role", "nakes")
			c.Set("auth_user_id", claims.NakesID)
			return next(c)
		}
	}
}

func extractBearerToken(c *echo.Context) string {
	auth := c.Request().Header.Get("Authorization")
	if !strings.HasPrefix(auth, "Bearer ") {
		return ""
	}
	return strings.TrimPrefix(auth, "Bearer ")
}
