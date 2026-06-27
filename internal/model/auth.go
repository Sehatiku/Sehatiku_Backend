package model

import "github.com/golang-jwt/jwt/v5"

// ── Request DTOs ──────────────────────────────────────────────────────────────

type FaskesRegisterRequest struct {
	Name        string `json:"name"         validate:"required"`
	Type        string `json:"type"         validate:"required,oneof=puskesmas klinik"`
	Address     string `json:"address"      validate:"required"`
	Region      string `json:"region"       validate:"required"`
	Username    string `json:"username"     validate:"required,min=4,max=50"`
	Password    string `json:"password"     validate:"required,min=8"`
	PhoneNumber string `json:"phone_number" validate:"required"`
}

type FaskesLoginRequest struct {
	Username string `json:"username" validate:"required"`
	Password string `json:"password" validate:"required"`
}

type NakesLoginRequest struct {
	Username string `json:"username" validate:"required"`
	Password string `json:"password" validate:"required"`
}

type PatientLoginRequest struct {
	Username string `json:"username" validate:"required"`
	Password string `json:"password" validate:"required"`
}

type RefreshTokenRequest struct {
	RefreshToken string `json:"refresh_token" validate:"required"`
}

type LogoutRequest struct {
	RefreshToken string `json:"refresh_token" validate:"required"`
}

// ── Response DTOs ─────────────────────────────────────────────────────────────

type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	ExpiresIn    int    `json:"expires_in"`
}

type FaskesLoginResponse struct {
	Token    TokenResponse `json:"token"`
	FaskesID string        `json:"faskes_id"`
	Name     string        `json:"name"`
}

type NakesLoginResponse struct {
	Token    TokenResponse `json:"token"`
	NakesID  string        `json:"nakes_id"`
	FaskesID string        `json:"faskes_id"`
	FullName string        `json:"full_name"`
	Role     string        `json:"role"`
}

type PatientLoginResponse struct {
	Token     TokenResponse `json:"token"`
	PatientID string        `json:"patient_id"`
	FaskesID  string        `json:"faskes_id"`
	FullName  string        `json:"full_name"`
}

// ── JWT Claims ────────────────────────────────────────────────────────────────

type FaskesAuthClaims struct {
	Typ      string `json:"typ"`
	FaskesID string `json:"faskes_id"`
	jwt.RegisteredClaims
}

type NakesAuthClaims struct {
	Typ      string `json:"typ"`
	NakesID  string `json:"nakes_id"`
	FaskesID string `json:"faskes_id"`
	Role     string `json:"role"`
	jwt.RegisteredClaims
}

type PatientAuthClaims struct {
	Typ       string `json:"typ"`
	PatientID string `json:"patient_id"`
	FaskesID  string `json:"faskes_id"`
	jwt.RegisteredClaims
}
