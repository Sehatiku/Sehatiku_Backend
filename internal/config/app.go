package config

import (
	"sehatiku-backend/internal/delivery/http/controller"
	"sehatiku-backend/internal/delivery/http/routing"
	ocrgw "sehatiku-backend/internal/gateway/ocr"
	"sehatiku-backend/internal/gateway/whatsapp"
	"sehatiku-backend/internal/helper"
	"sehatiku-backend/internal/repository"
	"sehatiku-backend/internal/usecase"

	"github.com/go-playground/validator/v10"
	"github.com/labstack/echo/v5"
	"github.com/redis/go-redis/v9"
	"github.com/spf13/viper"
	"go.uber.org/zap"
	"gorm.io/gorm"
)

type BootStrapConfig struct {
	DB       *gorm.DB
	App      *echo.Echo
	Log      *zap.Logger
	Validate *validator.Validate
	Config   *viper.Viper
	Redis    *redis.Client
	JWT      *helper.JWTHelper
	WhatsApp *whatsapp.WhatsAppGateway
}

func BootStrap(config *BootStrapConfig) {
	// Repositories
	faskesRepo := &repository.FaskesRepository{}
	nakesRepo := &repository.NakesRepository{}
	patientRepo := &repository.PatientRepository{}
	sessionRepo := &repository.SessionRepository{
		Redis: config.Redis,
		Log:   config.Log,
	}

	// Gateways
	ktpOCRBaseURL := config.Config.GetString("KTP_OCR_BASE_URL")
	if ktpOCRBaseURL == "" {
		ktpOCRBaseURL = "https://use.api.co.id"
	}
	ktpOCRGateway := ocrgw.New(
		config.Config.GetString("KTP_OCR_API_KEY"),
		ktpOCRBaseURL,
		config.Log,
	)

	// Use Cases
	faskesAuthUC := &usecase.FaskesAuthUseCase{
		DB:          config.DB,
		FaskesRepo:  faskesRepo,
		SessionRepo: sessionRepo,
		JWT:         config.JWT,
		WhatsApp:    config.WhatsApp,
		Log:         config.Log,
	}
	nakesAuthUC := &usecase.NakesAuthUseCase{
		DB:          config.DB,
		NakesRepo:   nakesRepo,
		SessionRepo: sessionRepo,
		JWT:         config.JWT,
		WhatsApp:    config.WhatsApp,
		Log:         config.Log,
	}
	patientAuthUC := &usecase.PatientAuthUseCase{
		DB:          config.DB,
		PatientRepo: patientRepo,
		SessionRepo: sessionRepo,
		JWT:         config.JWT,
		WhatsApp:    config.WhatsApp,
		Log:         config.Log,
	}
	tokenUC := &usecase.TokenUseCase{
		SessionRepo: sessionRepo,
		JWT:         config.JWT,
		Log:         config.Log,
	}
	nakesRegUC := &usecase.NakesRegistrationUseCase{
		DB:         config.DB,
		NakesRepo:  nakesRepo,
		OCRGateway: ktpOCRGateway,
		WhatsApp:   config.WhatsApp,
		Log:        config.Log,
	}
	nakesUC := &usecase.NakesUseCase{
		DB:        config.DB,
		NakesRepo: nakesRepo,
		Log:       config.Log,
	}
	patientUC := &usecase.PatientUseCase{
		DB:          config.DB,
		PatientRepo: patientRepo,
		NakesRepo:   nakesRepo,
		Log:         config.Log,
	}
	patientRegUC := &usecase.PatientRegistrationUseCase{
		DB:          config.DB,
		PatientRepo: patientRepo,
		NakesRepo:   nakesRepo,
		OCRGateway:  ktpOCRGateway,
		WhatsApp:    config.WhatsApp,
		Log:         config.Log,
	}
	dashboardRepo := &repository.DashboardRepository{}
	dashboardUC := &usecase.DashboardUseCase{
		DB:            config.DB,
		DashboardRepo: dashboardRepo,
		Log:           config.Log,
	}
	patientDashboardRepo := &repository.PatientDashboardRepository{}
	patientDashboardUC := &usecase.PatientDashboardUseCase{
		DB:          config.DB,
		Repo:        patientDashboardRepo,
		PatientRepo: patientRepo,
		Log:         config.Log,
	}

	// Controllers
	faskesAuthCtrl := &controller.FaskesAuthController{UseCase: faskesAuthUC}
	nakesAuthCtrl := &controller.NakesAuthController{UseCase: nakesAuthUC}
	patientAuthCtrl := &controller.PatientAuthController{UseCase: patientAuthUC}
	tokenCtrl := &controller.TokenController{UseCase: tokenUC}
	nakesRegCtrl := &controller.NakesRegistrationController{UseCase: nakesRegUC}
	nakesCtrl := &controller.NakesController{UseCase: nakesUC}
	patientCtrl := &controller.PatientController{UseCase: patientUC}
	patientRegCtrl := &controller.PatientRegistrationController{UseCase: patientRegUC}
	dashboardCtrl := &controller.DashboardController{
		SummaryUseCase: dashboardUC,
		QueueUseCase:   dashboardUC,
	}
	patientDashboardCtrl := &controller.PatientDashboardController{UseCase: patientDashboardUC}

	config.App.Validator = &CustomValidator{validator: config.Validate}

	routeConfig := routing.RouteConfig{
		App:                           config.App,
		JWTHelper:                     config.JWT,
		FaskesAuthController:          faskesAuthCtrl,
		NakesAuthController:           nakesAuthCtrl,
		PatientAuthController:         patientAuthCtrl,
		TokenController:               tokenCtrl,
		NakesRegistrationController:   nakesRegCtrl,
		NakesController:               nakesCtrl,
		PatientController:             patientCtrl,
		PatientRegistrationController: patientRegCtrl,
		DashboardController:           dashboardCtrl,
		PatientDashboardController:    patientDashboardCtrl,
	}
	routeConfig.SetUp()
}
