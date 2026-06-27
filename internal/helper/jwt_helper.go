package helper

import (
	"fmt"
	"sehatiku-backend/internal/model"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"go.uber.org/zap"
)

const accessTokenTTL = 12 * time.Hour

type JWTHelper struct {
	Secret string
	Log    *zap.Logger
}

func (h *JWTHelper) GenerateFaskesToken(faskesID string) (string, error) {
	claims := model.FaskesAuthClaims{
		Typ:      "faskes",
		FaskesID: faskesID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(accessTokenTTL)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(h.Secret))
	if err != nil {
		return "", fmt.Errorf("signing faskes token: %w", err)
	}
	return signed, nil
}

func (h *JWTHelper) GenerateNakesToken(nakesID, faskesID, role string) (string, error) {
	claims := model.NakesAuthClaims{
		Typ:      "nakes",
		NakesID:  nakesID,
		FaskesID: faskesID,
		Role:     role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(accessTokenTTL)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(h.Secret))
	if err != nil {
		return "", fmt.Errorf("signing nakes token: %w", err)
	}
	return signed, nil
}

func (h *JWTHelper) GeneratePatientToken(patientID, faskesID string) (string, error) {
	claims := model.PatientAuthClaims{
		Typ:       "patient",
		PatientID: patientID,
		FaskesID:  faskesID,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(accessTokenTTL)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	signed, err := token.SignedString([]byte(h.Secret))
	if err != nil {
		return "", fmt.Errorf("signing patient token: %w", err)
	}
	return signed, nil
}

func (h *JWTHelper) ValidateFaskesToken(raw string) (*model.FaskesAuthClaims, error) {
	claims := &model.FaskesAuthClaims{}
	_, err := jwt.ParseWithClaims(raw, claims, h.keyFunc)
	if err != nil {
		return nil, fmt.Errorf("validating faskes token: %w", err)
	}
	if claims.Typ != "faskes" {
		return nil, fmt.Errorf("token type mismatch: expected faskes, got %s", claims.Typ)
	}
	return claims, nil
}

func (h *JWTHelper) ValidateNakesToken(raw string) (*model.NakesAuthClaims, error) {
	claims := &model.NakesAuthClaims{}
	_, err := jwt.ParseWithClaims(raw, claims, h.keyFunc)
	if err != nil {
		return nil, fmt.Errorf("validating nakes token: %w", err)
	}
	if claims.Typ != "nakes" {
		return nil, fmt.Errorf("token type mismatch: expected nakes, got %s", claims.Typ)
	}
	return claims, nil
}

func (h *JWTHelper) ValidatePatientToken(raw string) (*model.PatientAuthClaims, error) {
	claims := &model.PatientAuthClaims{}
	_, err := jwt.ParseWithClaims(raw, claims, h.keyFunc)
	if err != nil {
		return nil, fmt.Errorf("validating patient token: %w", err)
	}
	if claims.Typ != "patient" {
		return nil, fmt.Errorf("token type mismatch: expected patient, got %s", claims.Typ)
	}
	return claims, nil
}

func (h *JWTHelper) keyFunc(_ *jwt.Token) (any, error) {
	return []byte(h.Secret), nil
}

func AccessTokenTTLSeconds() int {
	return int(accessTokenTTL.Seconds())
}
