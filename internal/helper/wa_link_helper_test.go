package helper

import (
	"net/url"
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

func TestBuildWarmupInviteText(t *testing.T) {
	const botLink = "https://wa.me/62812345678?text=HALO"
	const password = "secret12" // tidak boleh pernah muncul di teks apa pun

	t.Run("empty bot link returns empty text", func(t *testing.T) {
		got := BuildWarmupInviteText(entity.RecipientRolePatient, "Budi", "", "budi", "")
		if got != "" {
			t.Errorf("expected empty text when bot link is empty, got %q", got)
		}
	})

	t.Run("patient text has username and bot link, no password", func(t *testing.T) {
		got := BuildWarmupInviteText(entity.RecipientRolePatient, "Budi", "", "budi", botLink)
		if !strings.Contains(got, "budi") {
			t.Errorf("patient text missing username; got %q", got)
		}
		if !strings.Contains(got, botLink) {
			t.Errorf("patient text missing bot link; got %q", got)
		}
		if strings.Contains(got, password) {
			t.Errorf("patient text must NOT contain password; got %q", got)
		}
	})

	t.Run("nakes text uses same template as patient", func(t *testing.T) {
		got := BuildWarmupInviteText(entity.RecipientRoleNakes, "dr. Sari", "", "sari", botLink)
		if !strings.Contains(got, "sari") || !strings.Contains(got, botLink) {
			t.Errorf("nakes text missing username/bot link; got %q", got)
		}
		if strings.Contains(got, password) {
			t.Errorf("nakes text must NOT contain password; got %q", got)
		}
	})

	t.Run("companion text has patient name and bot link, no username", func(t *testing.T) {
		got := BuildWarmupInviteText(entity.RecipientRoleCompanion, "Ibu Budi", "Budi", "budi", botLink)
		if !strings.Contains(got, "Budi") {
			t.Errorf("companion text missing patient name; got %q", got)
		}
		if !strings.Contains(got, botLink) {
			t.Errorf("companion text missing bot link; got %q", got)
		}
		if strings.Contains(got, "Username") || strings.Contains(got, "username") {
			t.Errorf("companion text must NOT mention username; got %q", got)
		}
		if strings.Contains(got, password) {
			t.Errorf("companion text must NOT contain password; got %q", got)
		}
	})
}

func TestBuildDirectInviteLink(t *testing.T) {
	const botLink = "https://wa.me/62812345678?text=HALO"

	t.Run("empty bot link returns empty direct link", func(t *testing.T) {
		got := BuildDirectInviteLink("081111111111", entity.RecipientRolePatient, "Budi", "", "budi", "")
		if got != "" {
			t.Errorf("expected empty direct link when bot link is empty, got %q", got)
		}
	})

	t.Run("empty recipient phone returns empty direct link", func(t *testing.T) {
		got := BuildDirectInviteLink("", entity.RecipientRolePatient, "Budi", "", "budi", botLink)
		if got != "" {
			t.Errorf("expected empty direct link when phone is empty, got %q", got)
		}
	})

	t.Run("points at normalized recipient number and embeds bot link in text", func(t *testing.T) {
		got := BuildDirectInviteLink("081111111111", entity.RecipientRolePatient, "Budi", "", "budi", botLink)
		// Link harus menunjuk ke nomor penerima (dinormalkan 0811… → 6281…), bukan bot.
		if !strings.HasPrefix(got, "https://wa.me/6281111111111?text=") {
			t.Errorf("direct link must target normalized recipient number; got %q", got)
		}
		// Bot link tertanam di dalam teks (ter-url-encode).
		if !strings.Contains(got, url.QueryEscape(botLink)) {
			t.Errorf("direct link text must embed bot link; got %q", got)
		}
	})
}
