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
	FaskesController              *controller.FaskesController
	NakesAuthController           *controller.NakesAuthController
	PatientAuthController         *controller.PatientAuthController
	TokenController               *controller.TokenController
	NakesRegistrationController   *controller.NakesRegistrationController
	NakesController               *controller.NakesController
	PatientController             *controller.PatientController
	PatientRegistrationController *controller.PatientRegistrationController
	DashboardController           *controller.DashboardController
	PatientDashboardController    *controller.PatientDashboardController
	HealthLogController           *controller.HealthLogController
	HealthScoreController         *controller.HealthScoreController
	AssignedNakesController       *controller.AssignedNakesController
	ConsultationController        *controller.ConsultationController
	RecordController              *controller.RecordController
	PatientNotificationController *controller.PatientNotificationController
	SummaryController             *controller.SummaryController
	BaselineController            *controller.BaselineController
	EscalationController          *controller.EscalationController
}

func (r *RouteConfig) SetUp() {
	r.SetupFaskesGuestRoute()
	r.SetupFaskesAuthedRoute()
	r.SetupNakesGuestRoute()
	r.SetupNakesAuthedRoute()
	r.SetupPatientGuestRoute()
	r.SetupPatientAuthedRoute()
	r.SetupTokenRoute()
}

func (r *RouteConfig) SetupFaskesGuestRoute() {
	g := r.App.Group("/api/v1/faskes/auth")
	g.POST("/register", r.FaskesAuthController.Register)
	g.POST("/login", r.FaskesAuthController.Login)
}

func (r *RouteConfig) SetupFaskesAuthedRoute() {
	g := r.App.Group("/api/v1/faskes", middleware.FaskesAuth(r.JWTHelper))
	g.GET("/profile", r.FaskesController.GetProfile)
	g.GET("/nakes", r.NakesController.ListNakes)
	g.GET("/nakes/:id", r.NakesController.GetNakesDetail)
	g.POST("/nakes/register/ktp-ocr", r.NakesRegistrationController.ScanKTP)
	g.POST("/nakes/register", r.NakesRegistrationController.RegisterNakes)
	g.PATCH("/nakes/:id/status", r.NakesController.UpdateStatus)
	g.GET("/patients", r.PatientController.ListPatients)
	g.GET("/patients/:id", r.PatientController.GetPatientDetail)
	g.POST("/patients/register/ktp-ocr", r.PatientRegistrationController.ScanKTP)
	g.POST("/patients/register", r.PatientRegistrationController.RegisterPatient)
	g.GET("/patients/:id/baseline", r.BaselineController.GetLatest)
	g.POST("/patients/:id/baseline", r.BaselineController.Create)
	g.GET("/patients/:id/baseline/history", r.BaselineController.GetHistory)
}

func (r *RouteConfig) SetupNakesGuestRoute() {
	g := r.App.Group("/api/v1/nakes/auth")
	g.POST("/login", r.NakesAuthController.Login)
}

func (r *RouteConfig) SetupNakesAuthedRoute() {
	g := r.App.Group("/api/v1/nakes", middleware.NakesAuth(r.JWTHelper))
	g.GET("/profile", r.NakesController.GetMyProfile)
	g.GET("/dashboard/summary", r.DashboardController.GetSummary)
	g.GET("/dashboard/patient-queue", r.DashboardController.GetPatientQueue)
	g.GET("/consultations", r.ConsultationController.GetNakesList)
	g.POST("/consultations/:id/reply", r.ConsultationController.Reply)
	g.GET("/patients/:id/summary", r.SummaryController.GetNakesPatientSummary)
	g.GET("/escalations", r.EscalationController.GetQueue)
	g.PATCH("/escalations/:id/view", r.EscalationController.View)
	g.PATCH("/escalations/:id/act", r.EscalationController.Act)
	g.PATCH("/escalations/:id/dismiss", r.EscalationController.Dismiss)
}

func (r *RouteConfig) SetupPatientGuestRoute() {
	g := r.App.Group("/api/v1/patients/auth")
	g.POST("/login", r.PatientAuthController.Login)
}

func (r *RouteConfig) SetupPatientAuthedRoute() {
	g := r.App.Group("/api/v1/patients", middleware.PatientAuth(r.JWTHelper))
	g.GET("/dashboard", r.PatientDashboardController.GetDashboard)
	g.GET("/summary", r.SummaryController.GetPatientSummary)
	g.POST("/health-logs", r.HealthLogController.Create)
	g.GET("/health-score", r.HealthScoreController.Get)
	g.GET("/assigned-nakes", r.AssignedNakesController.GetAssignedNakes)
	g.POST("/consultations", r.ConsultationController.Create)
	g.GET("/consultations", r.ConsultationController.GetPatientList)
	g.POST("/records", r.RecordController.Create)
	g.GET("/records/history", r.RecordController.GetHistory)
	g.GET("/records/today-status", r.RecordController.GetTodayStatus)
	g.GET("/baseline/history", r.BaselineController.GetMyHistory)
	g.GET("/notifications", r.PatientNotificationController.GetNotifications)
	g.GET("/notifications/unread-count", r.PatientNotificationController.GetUnreadCount)
	g.POST("/notifications/read-all", r.PatientNotificationController.MarkAllRead)
	g.PATCH("/notifications/:id/read", r.PatientNotificationController.MarkRead)
}

func (r *RouteConfig) SetupTokenRoute() {
	g := r.App.Group("/api/v1/auth")
	g.POST("/refresh", r.TokenController.Refresh)
	g.POST("/logout", r.TokenController.Logout, middleware.AnyAuth(r.JWTHelper))
}
