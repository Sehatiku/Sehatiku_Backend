package model

// RegisterDeviceTokenRequest — body POST /api/v1/patients/device-tokens.
type RegisterDeviceTokenRequest struct {
	Token    string `json:"token" validate:"required"`
	Platform string `json:"platform" validate:"required,oneof=android ios"`
}

// DeregisterDeviceTokenRequest — body DELETE /api/v1/patients/device-tokens.
type DeregisterDeviceTokenRequest struct {
	Token string `json:"token" validate:"required"`
}
