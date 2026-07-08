package config

import (
	"sehatiku-backend/internal/delivery/http/controller"
	"sehatiku-backend/internal/delivery/http/routing"
	wadelivery "sehatiku-backend/internal/delivery/whatsapp"
	geminigw "sehatiku-backend/internal/gateway/gemini"
	mlgw "sehatiku-backend/internal/gateway/ml"
	ocrgw "sehatiku-backend/internal/gateway/ocr"
	"sehatiku-backend/internal/gateway/push"
	"sehatiku-backend/internal/gateway/whatsapp"
	"sehatiku-backend/internal/helper"
	"sehatiku-backend/internal/repository"
	"sehatiku-backend/internal/usecase"
	"time"

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
	Push     *push.PushGateway
}

func BootStrap(config *BootStrapConfig) {
	// Repositories
	faskesRepo := &repository.FaskesRepository{}
	nakesRepo := &repository.NakesRepository{}
	patientRepo := &repository.PatientRepository{}
	notificationRepo := &repository.NotificationRepository{}
	patientNotificationRepo := &repository.PatientNotificationRepository{}
	sessionRepo := &repository.SessionRepository{
		Redis: config.Redis,
		Log:   config.Log,
	}
	pendingCredentialRepo := &repository.PendingCredentialRepository{
		Redis: config.Redis,
		Log:   config.Log,
	}
	patientClinicalBaselineRepo := &repository.PatientClinicalBaselineRepository{}
	escalationRepo := &repository.EscalationRepository{}
	devicePushTokenRepo := &repository.DevicePushTokenRepository{}
	phoneRepo := &repository.PhoneRepository{}

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

	// ML inference service (NER+TKPI /extract, XGBoost+SHAP /predict_health_score),
	// dideploy terpisah (HF Space). ML_API_KEY dikirim sebagai header X-API-Key.
	mlGateway := mlgw.New(
		config.Config.GetString("ML_API_BASE_URL"),
		config.Config.GetString("ML_API_KEY"),
		config.Log,
	)

	// Gemini generative API — menyusun narasi ringkasan kesehatan (endpoint summary).
	geminiGateway := geminigw.New(
		config.Config.GetString("GEMINI_API_KEY"),
		config.Config.GetString("GEMINI_MODEL"), // kosong -> default gemini-2.5-flash
		config.Log,
	)

	// Gateway Gemini TERPISAH untuk OCR baseline (register/baseline-ocr) memakai key/model
	// sendiri agar kuota & billing tidak bercampur dengan summary. Bila GEMINI_OCR_API_KEY
	// kosong, fallback ke key/model summary supaya OCR tetap jalan tanpa konfigurasi tambahan.
	geminiOCRKey := config.Config.GetString("GEMINI_OCR_API_KEY")
	geminiOCRModel := config.Config.GetString("GEMINI_OCR_MODEL")
	if geminiOCRKey == "" {
		geminiOCRKey = config.Config.GetString("GEMINI_API_KEY")
		geminiOCRModel = config.Config.GetString("GEMINI_MODEL")
	}
	geminiOCRGateway := geminigw.New(geminiOCRKey, geminiOCRModel, config.Log)

	// Use Cases
	faskesAuthUC := &usecase.FaskesAuthUseCase{
		DB:          config.DB,
		FaskesRepo:  faskesRepo,
		SessionRepo: sessionRepo,
		PhoneRepo:   phoneRepo,
		JWT:         config.JWT,
		Log:         config.Log,
	}
	nakesAuthUC := &usecase.NakesAuthUseCase{
		DB:          config.DB,
		NakesRepo:   nakesRepo,
		SessionRepo: sessionRepo,
		JWT:         config.JWT,
		Log:         config.Log,
	}
	patientAuthUC := &usecase.PatientAuthUseCase{
		DB:          config.DB,
		PatientRepo: patientRepo,
		SessionRepo: sessionRepo,
		JWT:         config.JWT,
		Log:         config.Log,
	}
	tokenUC := &usecase.TokenUseCase{
		SessionRepo: sessionRepo,
		JWT:         config.JWT,
		Log:         config.Log,
	}
	nakesRegUC := &usecase.NakesRegistrationUseCase{
		DB:                config.DB,
		NakesRepo:         nakesRepo,
		FaskesRepo:        faskesRepo,
		PhoneRepo:         phoneRepo,
		NotificationRepo:  notificationRepo,
		PendingCredential: pendingCredentialRepo,
		OCRGateway:        ktpOCRGateway,
		WhatsApp:          config.WhatsApp,
		Log:               config.Log,
	}
	faskesUC := &usecase.FaskesUseCase{
		DB:         config.DB,
		FaskesRepo: faskesRepo,
		Log:        config.Log,
	}
	nakesUC := &usecase.NakesUseCase{
		DB:        config.DB,
		NakesRepo: nakesRepo,
		Log:       config.Log,
	}
	patientDashboardRepo := &repository.PatientDashboardRepository{}
	riskScoreRepo := &repository.RiskScoreRepository{}

	patientUC := &usecase.PatientUseCase{
		DB:            config.DB,
		PatientRepo:   patientRepo,
		NakesRepo:     nakesRepo,
		BaselineRepo:  patientClinicalBaselineRepo,
		HistoryRepo:   patientDashboardRepo,
		RiskScoreRepo: riskScoreRepo,
		Log:           config.Log,
	}
	patientRegUC := &usecase.PatientRegistrationUseCase{
		DB:                config.DB,
		PatientRepo:       patientRepo,
		NakesRepo:         nakesRepo,
		PhoneRepo:         phoneRepo,
		NotificationRepo:  notificationRepo,
		PendingCredential: pendingCredentialRepo,
		BaselineRepo:      patientClinicalBaselineRepo,
		OCRGateway:        ktpOCRGateway,
		Gemini:            geminiOCRGateway,
		WhatsApp:          config.WhatsApp,
		Log:               config.Log,
	}

	// Warm-up WhatsApp: handler pesan masuk yang mengirim kredensial saat penerima
	// menghubungi bot lebih dulu (mengatasi blok kontak-dingin / error 463).
	waInboundUC := &usecase.WAInboundUseCase{
		DB:                config.DB,
		PendingCredential: pendingCredentialRepo,
		WhatsApp:          config.WhatsApp,
		NotificationRepo:  notificationRepo,
		Log:               config.Log,
	}
	inboundHandler := wadelivery.NewInboundHandler(waInboundUC, config.Log)
	inboundHandler.Register(config.WhatsApp.Client)
	dashboardRepo := &repository.DashboardRepository{}
	dashboardUC := &usecase.DashboardUseCase{
		DB:            config.DB,
		DashboardRepo: dashboardRepo,
		Log:           config.Log,
	}

	patientDashboardUC := &usecase.PatientDashboardUseCase{
		DB:          config.DB,
		Repo:        patientDashboardRepo,
		PatientRepo: patientRepo,
		Log:         config.Log,
	}
	consultationRepo := &repository.ConsultationRepository{}
	healthLogRepo := &repository.HealthLogRepository{}
	healthLogGuardRepo := &repository.HealthLogGuardRepository{
		Redis: config.Redis,
		Log:   config.Log,
	}
	// Input data harian via WhatsApp: pasien/pendamping kirim teks ("gula 180",
	// "tensi 120/80", dll) dan bot membalas konfirmasi atau panduan format.
	// Harus setelah healthLogRepo dideklarasi; inboundHandler juga sudah siap di atas.
	waHealthLogUC := &usecase.WAHealthLogUseCase{
		DB:          config.DB,
		PatientRepo: patientRepo,
		NakesRepo:   nakesRepo,
		LogRepo:     healthLogRepo,
		Extractor:   mlGateway, // enrichment makanan via NER+TKPI (opsional)
		WhatsApp:    config.WhatsApp,
		Log:         config.Log,
	}
	inboundHandler.HealthLogUC = waHealthLogUC
	healthLogUC := &usecase.HealthLogUseCase{
		DB:            config.DB,
		HealthLogRepo: healthLogRepo,
		GuardRepo:     healthLogGuardRepo,
		Extractor:     mlGateway, // makanan di-enrich lewat NER+TKPI saat dicatat
		Log:           config.Log,
	}

	// Skoring harian: roll-7 (SQL) -> daily_features -> ML /predict -> risk_scores.
	dailyFeatureRepo := &repository.DailyFeatureRepository{}

	scoringUC := &usecase.ScoringUseCase{
		DB:               config.DB,
		DailyFeatureRepo: dailyFeatureRepo,
		RiskScoreRepo:    riskScoreRepo,
		PatientRepo:      patientRepo,
		BaselineRepo:     patientClinicalBaselineRepo,
		ML:               mlGateway,
		Log:              config.Log,
	}
	alertBudget := config.Config.GetInt("ALERT_BUDGET")
	if alertBudget == 0 {
		alertBudget = 20 // default harian per nakes; 0 di env = pakai default ini
	}
	acuteCooldownHours := config.Config.GetInt("ACUTE_COOLDOWN_HOURS")
	if acuteCooldownHours <= 0 {
		acuteCooldownHours = 24
	}
	acuteGlucoseHigh := config.Config.GetFloat64("ACUTE_GLUCOSE_HIGH")
	if acuteGlucoseHigh <= 0 {
		acuteGlucoseHigh = 300
	}
	acuteGlucoseLow := config.Config.GetFloat64("ACUTE_GLUCOSE_LOW")
	if acuteGlucoseLow <= 0 {
		acuteGlucoseLow = 54
	}
	acuteSystolicHigh := config.Config.GetFloat64("ACUTE_SYSTOLIC_HIGH")
	if acuteSystolicHigh <= 0 {
		acuteSystolicHigh = 180
	}
	acuteDiastolicHigh := config.Config.GetFloat64("ACUTE_DIASTOLIC_HIGH")
	if acuteDiastolicHigh <= 0 {
		acuteDiastolicHigh = 120
	}
	escalationUC := &usecase.EscalationUseCase{
		DB:                 config.DB,
		Repo:               escalationRepo,
		RiskRepo:           riskScoreRepo,
		NakesRepo:          nakesRepo,
		WA:                 config.WhatsApp,
		NotifRepo:          notificationRepo,
		InboxRepo:          patientNotificationRepo,
		AlertBudget:        alertBudget,
		HealthLogRepo:      healthLogRepo,
		AcuteCooldown:      time.Duration(acuteCooldownHours) * time.Hour,
		AcuteGlucoseHigh:   acuteGlucoseHigh,
		AcuteGlucoseLow:    acuteGlucoseLow,
		AcuteSystolicHigh:  acuteSystolicHigh,
		AcuteDiastolicHigh: acuteDiastolicHigh,
		Log:                config.Log,
	}
	// Acute escalation hook — scoring fires it fire-and-forget after persisting a risk score.
	scoringUC.Escalation = escalationUC
	pushUC := &usecase.PushUseCase{
		DB:        config.DB,
		TokenRepo: devicePushTokenRepo,
		Gateway:   config.Push,
		Log:       config.Log,
	}
	escalationUC.Push = pushUC
	devicePushTokenUC := &usecase.DevicePushTokenUseCase{
		DB:   config.DB,
		Repo: devicePushTokenRepo,
		Log:  config.Log,
	}
	assignedNakesUC := &usecase.AssignedNakesUseCase{
		DB:          config.DB,
		PatientRepo: patientRepo,
		NakesRepo:   nakesRepo,
		Log:         config.Log,
	}
	consultationUC := &usecase.ConsultationUseCase{
		DB:          config.DB,
		Repo:        consultationRepo,
		PatientRepo: patientRepo,
		NakesRepo:   nakesRepo,
		InboxRepo:   patientNotificationRepo,
		Push:        pushUC,
		Log:         config.Log,
	}
	recordUC := &usecase.RecordUseCase{
		DB:          config.DB,
		LogRepo:     healthLogRepo,
		HistoryRepo: patientDashboardRepo,
		Extractor:   mlGateway, // enrich 'meals' lewat NER+TKPI
		Scorer:      scoringUC, // kembalikan health score di response /records
		Log:         config.Log,
	}
	patientNotificationUC := &usecase.PatientNotificationUseCase{
		DB:        config.DB,
		NotifRepo: patientNotificationRepo,
		Log:       config.Log,
	}
	summaryRepo := &repository.SummaryRepository{}
	summaryUC := &usecase.SummaryUseCase{
		DB:          config.DB,
		Repo:        summaryRepo,
		PatientRepo: patientRepo,
		Generator:   geminiGateway,
		Redis:       config.Redis,
		Log:         config.Log,
		// Dependensi tambahan Pre-Visit Brief (GET /nakes/patients/:id/brief).
		MedRepo:        summaryRepo,
		HistoryRepo:    patientDashboardRepo,
		RiskRepo:       riskScoreRepo,
		EscalationRepo: escalationRepo,
	}
	baselineUC := &usecase.PatientBaselineUseCase{
		DB:            config.DB,
		BaselineRepo:  patientClinicalBaselineRepo,
		PatientRepo:   patientRepo,
		NakesRepo:     nakesRepo,
		RiskScoreRepo: riskScoreRepo,
		Log:           config.Log,
	}

	// Controllers
	faskesAuthCtrl := &controller.FaskesAuthController{UseCase: faskesAuthUC}
	faskesCtrl := &controller.FaskesController{UseCase: faskesUC}
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
	healthLogCtrl := &controller.HealthLogController{UseCase: healthLogUC}
	healthScoreCtrl := &controller.HealthScoreController{UseCase: scoringUC}
	assignedNakesCtrl := &controller.AssignedNakesController{UseCase: assignedNakesUC}
	consultationCtrl := &controller.ConsultationController{UseCase: consultationUC}
	recordCtrl := &controller.RecordController{UseCase: recordUC}
	patientNotificationCtrl := &controller.PatientNotificationController{UseCase: patientNotificationUC}
	summaryCtrl := &controller.SummaryController{UseCase: summaryUC}
	baselineCtrl := &controller.BaselineController{UseCase: baselineUC}
	escalationCtrl := &controller.EscalationController{UseCase: escalationUC}
	devicePushTokenCtrl := &controller.DevicePushTokenController{UseCase: devicePushTokenUC}

	config.App.Validator = &CustomValidator{validator: config.Validate}

	routeConfig := routing.RouteConfig{
		App:                           config.App,
		JWTHelper:                     config.JWT,
		FaskesAuthController:          faskesAuthCtrl,
		FaskesController:              faskesCtrl,
		NakesAuthController:           nakesAuthCtrl,
		PatientAuthController:         patientAuthCtrl,
		TokenController:               tokenCtrl,
		NakesRegistrationController:   nakesRegCtrl,
		NakesController:               nakesCtrl,
		PatientController:             patientCtrl,
		PatientRegistrationController: patientRegCtrl,
		DashboardController:           dashboardCtrl,
		PatientDashboardController:    patientDashboardCtrl,
		HealthLogController:           healthLogCtrl,
		HealthScoreController:         healthScoreCtrl,
		AssignedNakesController:       assignedNakesCtrl,
		ConsultationController:        consultationCtrl,
		RecordController:              recordCtrl,
		PatientNotificationController: patientNotificationCtrl,
		SummaryController:             summaryCtrl,
		BaselineController:            baselineCtrl,
		EscalationController:          escalationCtrl,
		DevicePushTokenController:     devicePushTokenCtrl,
	}
	routeConfig.SetUp()
}
