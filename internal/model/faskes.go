package model

import "time"

// ── Faskes Profile (self view) ───────────────────────────────────────────────

type FaskesProfileResponse struct {
	FaskesID    string    `json:"faskes_id"`
	Name        string    `json:"name"`
	Type        string    `json:"type"`
	Address     string    `json:"address"`
	Region      string    `json:"region"`
	Username    string    `json:"username"`
	PhoneNumber string    `json:"phone_number"`
	Status      string    `json:"status"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}
