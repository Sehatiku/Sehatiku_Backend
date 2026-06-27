package main

import "sehatiku-backend/internal/config"

func main() {
	cfg := config.NewViper()
	log := config.NewLogger(cfg)
	db := config.ConnectDB(cfg, log)
	validate := config.NewValidator(cfg)
	redis := config.SetUpRedis(cfg, log)
	app := config.NewEcho(cfg)
	jwt := config.SetUpJWT(cfg, log)
	config.BootStrap(&config.BootStrapConfig{
		DB:       db,
		App:      app,
		Log:      log,
		Validate: validate,
		Config:   cfg,
		Redis:    redis,
		JWT:      jwt,
	})

	port := cfg.GetString("APP_PORT")
	if port == "" {
		port = "9000"
	}
	app.Start(":" + port)
}
