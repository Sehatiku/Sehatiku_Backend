package helper

import (
	"strings"
	"testing"

	"sehatiku-backend/internal/entity"
)

func TestBuildWAMeLink(t *testing.T) {
	tests := []struct {
		name     string
		botPhone string
		text     string
		want     string
	}{
		{
			name:     "phone and text",
			botPhone: "62812345678",
			text:     "HALO SEHATIKU",
			want:     "https://wa.me/62812345678?text=HALO+SEHATIKU",
		},
		{
			name:     "phone without text",
			botPhone: "62812345678",
			text:     "",
			want:     "https://wa.me/62812345678",
		},
		{
			name:     "empty phone returns empty link",
			botPhone: "",
			text:     "HALO SEHATIKU",
			want:     "",
		},
		{
			name:     "special characters are url-encoded",
			botPhone: "62812345678",
			text:     "HALO & SELAMAT?",
			want:     "https://wa.me/62812345678?text=HALO+%26+SELAMAT%3F",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BuildWAMeLink(tt.botPhone, tt.text)
			if got != tt.want {
				t.Errorf("BuildWAMeLink(%q, %q) = %q; want %q", tt.botPhone, tt.text, got, tt.want)
			}
		})
	}
}

func TestBuildWarmupShareMessage(t *testing.T) {
	const link = "https://wa.me/62812345678?text=HALO"
	const password = "secret12" // tidak boleh pernah muncul di pesan apa pun

	t.Run("empty link returns empty message", func(t *testing.T) {
		got := BuildWarmupShareMessage(entity.RecipientRolePatient, "Budi", "", "budi", "")
		if got != "" {
			t.Errorf("expected empty message when link is empty, got %q", got)
		}
	})

	t.Run("patient message has username and link, no password", func(t *testing.T) {
		got := BuildWarmupShareMessage(entity.RecipientRolePatient, "Budi", "", "budi", link)
		if !strings.Contains(got, "budi") {
			t.Errorf("patient message missing username; got %q", got)
		}
		if !strings.Contains(got, link) {
			t.Errorf("patient message missing link; got %q", got)
		}
		if strings.Contains(got, password) {
			t.Errorf("patient message must NOT contain password; got %q", got)
		}
	})

	t.Run("nakes message uses same template as patient", func(t *testing.T) {
		got := BuildWarmupShareMessage(entity.RecipientRoleNakes, "dr. Sari", "", "sari", link)
		if !strings.Contains(got, "sari") || !strings.Contains(got, link) {
			t.Errorf("nakes message missing username/link; got %q", got)
		}
		if strings.Contains(got, password) {
			t.Errorf("nakes message must NOT contain password; got %q", got)
		}
	})

	t.Run("companion message has patient name and link, no username", func(t *testing.T) {
		got := BuildWarmupShareMessage(entity.RecipientRoleCompanion, "Ibu Budi", "Budi", "budi", link)
		if !strings.Contains(got, "Budi") {
			t.Errorf("companion message missing patient name; got %q", got)
		}
		if !strings.Contains(got, link) {
			t.Errorf("companion message missing link; got %q", got)
		}
		if strings.Contains(got, "Username") || strings.Contains(got, "username") {
			t.Errorf("companion message must NOT mention username; got %q", got)
		}
		if strings.Contains(got, password) {
			t.Errorf("companion message must NOT contain password; got %q", got)
		}
	})
}
