package usecase

import (
	"testing"
	"time"

	"sehatiku-backend/internal/repository"
)

func TestSlopePerWeek(t *testing.T) {
	f := func(v float64) *float64 { return &v }

	tests := []struct {
		name   string
		points []trendPoint
		want   *float64 // nil = tren tak bisa dihitung
	}{
		{"kurang dari 3 titik", []trendPoint{{"2026-07-01", 100}, {"2026-07-02", 110}}, nil},
		{"naik 5/hari = 35/minggu", []trendPoint{
			{"2026-07-01", 100}, {"2026-07-02", 105}, {"2026-07-03", 110}, {"2026-07-04", 115},
		}, f(35)},
		{"flat = 0", []trendPoint{
			{"2026-07-01", 120}, {"2026-07-03", 120}, {"2026-07-06", 120},
		}, f(0)},
		{"gap tanggal tetap benar (turun 2/hari = -14/minggu)", []trendPoint{
			{"2026-07-01", 200}, {"2026-07-05", 192}, {"2026-07-11", 180},
		}, f(-14)},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := slopePerWeek(tt.points)
			switch {
			case tt.want == nil && got != nil:
				t.Errorf("slope = %v, want nil", *got)
			case tt.want != nil && got == nil:
				t.Errorf("slope = nil, want %v", *tt.want)
			case tt.want != nil && got != nil && *got != *tt.want:
				t.Errorf("slope = %v, want %v", *got, *tt.want)
			}
		})
	}
}

func TestParseBriefLLMOutput(t *testing.T) {
	tests := []struct {
		name          string
		raw           string
		wantNarrative string
		wantQuestions int
	}{
		{
			"JSON bersih",
			`{"narrative": "Gula naik.", "questions": ["Apakah lupa obat?", "Apa pemicunya?"]}`,
			"Gula naik.", 2,
		},
		{
			"JSON dalam code fence",
			"```json\n{\"narrative\": \"Tensi stabil.\", \"questions\": [\"Bagaimana tidurnya?\"]}\n```",
			"Tensi stabil.", 1,
		},
		{
			"teks bebas (bukan JSON) -> semua jadi narrative",
			"Pasien menunjukkan tren gula naik dalam 30 hari terakhir.",
			"Pasien menunjukkan tren gula naik dalam 30 hari terakhir.", 0,
		},
		{
			"JSON rusak -> fallback teks mentah",
			`{"narrative": "terpotong...`,
			`{"narrative": "terpotong...`, 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			narrative, questions := parseBriefLLMOutput(tt.raw)
			if narrative != tt.wantNarrative {
				t.Errorf("narrative = %q, want %q", narrative, tt.wantNarrative)
			}
			if len(questions) != tt.wantQuestions {
				t.Errorf("len(questions) = %d, want %d", len(questions), tt.wantQuestions)
			}
			if questions == nil {
				t.Error("questions harus slice kosong, bukan nil (JSON null di response)")
			}
		})
	}
}

func TestBuildBriefMedAdherence(t *testing.T) {
	day := func(dateStr string, taken bool) repository.MedDayRaw {
		d, _ := time.Parse("2006-01-02", dateStr)
		return repository.MedDayRaw{LogDate: d, Taken: taken}
	}

	// 2026-07-04 = Sabtu, 2026-07-05 = Minggu.
	out := buildBriefMedAdherence([]repository.MedDayRaw{
		day("2026-07-01", true),
		day("2026-07-02", true),
		day("2026-07-03", true),
		day("2026-07-04", false),
		day("2026-07-05", false),
	})

	if out.TakenDays != 3 || out.MissedDays != 2 {
		t.Errorf("taken/missed = %d/%d, want 3/2", out.TakenDays, out.MissedDays)
	}
	if out.AdherenceRatePct != 60 {
		t.Errorf("rate = %v, want 60", out.AdherenceRatePct)
	}
	if out.MissedWeekdays["Sabtu"] != 1 || out.MissedWeekdays["Minggu"] != 1 {
		t.Errorf("missed weekdays = %v, want Sabtu:1 Minggu:1", out.MissedWeekdays)
	}
	if len(out.MissedDates) != 2 || out.MissedDates[0] != "2026-07-04" {
		t.Errorf("missed dates = %v", out.MissedDates)
	}

	// Tanpa data sama sekali: rate 0, slice/map kosong (bukan nil).
	empty := buildBriefMedAdherence(nil)
	if empty.AdherenceRatePct != 0 || empty.MissedDates == nil || empty.MissedWeekdays == nil {
		t.Errorf("empty case: %+v", empty)
	}
}
