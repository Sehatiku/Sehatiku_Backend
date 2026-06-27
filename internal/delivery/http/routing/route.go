package routing

import (
	"sehatiku-backend/internal/delivery/http/controller"
	"sehatiku-backend/internal/delivery/http/middleware"
	"sehatiku-backend/internal/helper"

	"github.com/labstack/echo/v5"
)

type RouteConfig struct {
	App       *echo.Echo
	JWTHelper *helper.JWTHelper

	FaskesAuthController          *controller.FaskesAuthController
	NakesAuthController           *controller.NakesAuthController
	PatientAuthController         *controller.PatientAuthController
	TokenController               *controller.TokenController
	NakesRegistrationController   *controller.NakesRegistrationController
	PatientRegistrationController *controller.PatientRegistrationController
}

func (r *RouteConfig) SetUp() {
	r.SetupFaskesGuestRoute()
	r.SetupFaskesAuthedRoute()
	r.SetupNakesGuestRoute()
	r.SetupNakesAuthedRoute()
	r.SetupPatientGuestRoute()
	r.SetupTokenRoute()
}

func (r *RouteConfig) SetupFaskesGuestRoute() {
	g := r.App.Group("/api/v1/faskes/auth")
	g.POST("/register", r.FaskesAuthController.Register)
	g.POST("/login", r.FaskesAuthController.Login)
}

func (r *RouteConfig) SetupFaskesAuthedRoute() {
	g := r.App.Group("/api/v1/faskes", middleware.FaskesAuth(r.JWTHelper))
	g.POST("/nakes/register/ktp-ocr", r.NakesRegistrationController.ScanKTP)
	g.POST("/nakes/register", r.NakesRegistrationController.RegisterNakes)
}

func (r *RouteConfig) SetupNakesGuestRoute() {
	g := r.App.Group("/api/v1/nakes/auth")
	g.POST("/login", r.NakesAuthController.Login)
}

func (r *RouteConfig) SetupNakesAuthedRoute() {
	g := r.App.Group("/api/v1/nakes", middleware.NakesAuth(r.JWTHelper))
	g.POST("/patients/register/ktp-ocr", r.PatientRegistrationController.ScanKTP)
	g.POST("/patients/register", r.PatientRegistrationController.RegisterPatient)
}

func (r *RouteConfig) SetupPatientGuestRoute() {
	g := r.App.Group("/api/v1/patients/auth")
	g.POST("/login", r.PatientAuthController.Login)
}

func (r *RouteConfig) SetupTokenRoute() {
	g := r.App.Group("/api/v1/auth")
	g.POST("/refresh", r.TokenController.Refresh)
	g.POST("/logout", r.TokenController.Logout, middleware.AnyAuth(r.JWTHelper))
}
