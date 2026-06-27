package middleware

import (
	"net/http"
	"sehatiku-backend/internal/helper"
	"sehatiku-backend/internal/model"

	"github.com/golang-jwt/jwt/v5"
	"github.com/labstack/echo/v5"
)

// AnyAuth memvalidasi JWT dari faskes, nakes, atau patient dan men-set auth context.
// Dipakai untuk endpoint yang diakses oleh semua aktor (mis. logout, refresh).
func AnyAuth(jwtHelper *helper.JWTHelper) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c *echo.Context) error {
			raw := extractBearerToken(c)
			if raw == "" {
				return c.JSON(http.StatusUnauthorized, model.WebResponse[any]{
					Message: "unauthorized",
					Errors:  "authorization header diperlukan",
				})
			}

			typ, err := extractTypClaim(raw)
			if err != nil {
				return c.JSON(http.StatusUnauthorized, model.WebResponse[any]{
					Message: "unauthorized",
					Errors:  "token tidak valid",
				})
			}

			switch typ {
			case "faskes":
				claims, err := jwtHelper.ValidateFaskesToken(raw)
				if err != nil {
					break
				}
				c.Set("faskes_auth", claims)
				c.Set("auth_role", "faskes")
				c.Set("auth_user_id", claims.FaskesID)
				return next(c)
			case "nakes":
				claims, err := jwtHelper.ValidateNakesToken(raw)
				if err != nil {
					break
				}
				c.Set("nakes_auth", claims)
				c.Set("auth_role", "nakes")
				c.Set("auth_user_id", claims.NakesID)
				return next(c)
			case "patient":
				claims, err := jwtHelper.ValidatePatientToken(raw)
				if err != nil {
					break
				}
				c.Set("patient_auth", claims)
				c.Set("auth_role", "patient")
				c.Set("auth_user_id", claims.PatientID)
				return next(c)
			}

			return c.JSON(http.StatusUnauthorized, model.WebResponse[any]{
				Message: "unauthorized",
				Errors:  "token tidak valid atau sudah kadaluarsa",
			})
		}
	}
}

// extractTypClaim membaca klaim "typ" dari JWT tanpa memvalidasi signature.
func extractTypClaim(raw string) (string, error) {
	p := jwt.NewParser()
	token, _, err := p.ParseUnverified(raw, jwt.MapClaims{})
	if err != nil {
		return "", err
	}
	claims, ok := token.Claims.(jwt.MapClaims)
	if !ok {
		return "", jwt.ErrTokenInvalidClaims
	}
	typ, _ := claims["typ"].(string)
	return typ, nil
}
