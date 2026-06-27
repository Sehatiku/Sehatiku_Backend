package middleware

import (
	"net/http"
	"sehatiku-backend/internal/helper"
	"sehatiku-backend/internal/model"

	"github.com/labstack/echo/v5"
)

func PatientAuth(jwt *helper.JWTHelper) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			raw := extractBearerToken(c)
			if raw == "" {
				return c.JSON(http.StatusUnauthorized, model.WebResponse[any]{
					Message: "unauthorized",
					Errors:  "authorization header diperlukan",
				})
			}

			claims, err := jwt.ValidatePatientToken(raw)
			if err != nil {
				return c.JSON(http.StatusUnauthorized, model.WebResponse[any]{
					Message: "unauthorized",
					Errors:  "token tidak valid atau sudah kadaluarsa",
				})
			}

			c.Set("patient_auth", claims)
			c.Set("auth_role", "patient")
			c.Set("auth_user_id", claims.PatientID)
			return next(c)
		}
	}
}
