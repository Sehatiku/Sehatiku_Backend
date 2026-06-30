package helper

import (
	"testing"
)

func TestParseWAMessage_Glucose(t *testing.T) {
	cases := []struct {
		input string
		want  float64
	}{
		{"gula 180", 180},
		{"GULA DARAH 140", 140},
		{"GDS 200 mg/dl", 200},
		{"kadar gula 99", 99},
		{"gula darah saya 160", 160},
	}
	for _, c := range cases {
		got := ParseWAMessage(c.input)
		if got.Err != nil {
			t.Errorf("%q: unexpected error: %v", c.input, got.Err)
			continue
		}
		if got.MetricType != "glucose" {
			t.Errorf("%q: want metric glucose, got %s", c.input, got.MetricType)
			continue
		}
		if got.ValueNumeric == nil || *got.ValueNumeric != c.want {
			t.Errorf("%q: want %.0f, got %v", c.input, c.want, got.ValueNumeric)
		}
	}
}

func TestParseWAMessage_BP(t *testing.T) {
	cases := []struct {
		input    string
		wantSys  int
		wantDia  int
	}{
		{"tensi 120/80", 120, 80},
		{"TD 130/85", 130, 85},
		{"tekanan darah 140/90", 140, 90},
		{"bp 115/75", 115, 75},
		{"120/80", 120, 80},
	}
	for _, c := range cases {
		got := ParseWAMessage(c.input)
		if got.Err != nil {
			t.Errorf("%q: unexpected error: %v", c.input, got.Err)
			continue
		}
		if got.MetricType != "bp" {
			t.Errorf("%q: want metric bp, got %s", c.input, got.MetricType)
			continue
		}
		if got.BPSystolic == nil || *got.BPSystolic != c.wantSys {
			t.Errorf("%q: want sys %d, got %v", c.input, c.wantSys, got.BPSystolic)
		}
		if got.BPDiastolic == nil || *got.BPDiastolic != c.wantDia {
			t.Errorf("%q: want dia %d, got %v", c.input, c.wantDia, got.BPDiastolic)
		}
	}
}

func TestParseWAMessage_MedAdherence(t *testing.T) {
	cases := []struct {
		input string
		want  float64
	}{
		{"obat ya", 100},
		{"minum obat", 100},
		{"sudah minum obat", 100},
		{"obat tidak", 0},
		{"tidak minum obat", 0},
		{"lupa obat", 0},
	}
	for _, c := range cases {
		got := ParseWAMessage(c.input)
		if got.Err != nil {
			t.Errorf("%q: unexpected error: %v", c.input, got.Err)
			continue
		}
		if got.MetricType != "med_adherence" {
			t.Errorf("%q: want metric med_adherence, got %s", c.input, got.MetricType)
			continue
		}
		if got.ValueNumeric == nil || *got.ValueNumeric != c.want {
			t.Errorf("%q: want %.0f, got %v", c.input, c.want, got.ValueNumeric)
		}
	}
}

func TestParseWAMessage_Activity(t *testing.T) {
	cases := []struct {
		input string
		want  float64
	}{
		{"olahraga 30 menit", 30},
		{"jalan 45", 45},
		{"lari 20 menit", 20},
		{"senam 60 menit", 60},
		{"olahraga", 30}, // default fallback
	}
	for _, c := range cases {
		got := ParseWAMessage(c.input)
		if got.Err != nil {
			t.Errorf("%q: unexpected error: %v", c.input, got.Err)
			continue
		}
		if got.MetricType != "activity" {
			t.Errorf("%q: want metric activity, got %s", c.input, got.MetricType)
			continue
		}
		if got.ValueNumeric == nil || *got.ValueNumeric != c.want {
			t.Errorf("%q: want %.0f, got %v", c.input, c.want, got.ValueNumeric)
		}
	}
}

func TestParseWAMessage_Sleep(t *testing.T) {
	got := ParseWAMessage("tidur 7 jam")
	if got.Err != nil {
		t.Fatalf("unexpected error: %v", got.Err)
	}
	if got.MetricType != "sleep" || got.ValueNumeric == nil || *got.ValueNumeric != 7 {
		t.Errorf("got %+v; want sleep 7", got)
	}
}

func TestParseWAMessage_Stress(t *testing.T) {
	got := ParseWAMessage("stres 4")
	if got.Err != nil {
		t.Fatalf("unexpected error: %v", got.Err)
	}
	if got.MetricType != "stress" || got.ValueNumeric == nil || *got.ValueNumeric != 4 {
		t.Errorf("got %+v; want stress 4", got)
	}
}

func TestParseWAMessage_Weight(t *testing.T) {
	cases := []struct {
		input string
		want  float64
	}{
		{"berat 65 kg", 65},
		{"BB 70", 70},
		{"berat badan 68.5", 68.5},
		{"timbang 72 kg", 72},
	}
	for _, c := range cases {
		got := ParseWAMessage(c.input)
		if got.Err != nil {
			t.Errorf("%q: unexpected error: %v", c.input, got.Err)
			continue
		}
		if got.MetricType != "weight" {
			t.Errorf("%q: want metric weight, got %s", c.input, got.MetricType)
		}
		if got.ValueNumeric == nil || *got.ValueNumeric != c.want {
			t.Errorf("%q: want %.1f, got %v", c.input, c.want, got.ValueNumeric)
		}
	}
}

func TestParseWAMessage_Smoking(t *testing.T) {
	cases := []struct {
		input string
		want  float64
	}{
		{"rokok 5 batang", 5},
		{"merokok 3", 3},
		{"tidak rokok", 0},
		{"berhenti rokok", 0},
	}
	for _, c := range cases {
		got := ParseWAMessage(c.input)
		if got.Err != nil {
			t.Errorf("%q: unexpected error: %v", c.input, got.Err)
			continue
		}
		if got.MetricType != "smoking" {
			t.Errorf("%q: want metric smoking, got %s", c.input, got.MetricType)
		}
		if got.ValueNumeric == nil || *got.ValueNumeric != c.want {
			t.Errorf("%q: want %.0f, got %v", c.input, c.want, got.ValueNumeric)
		}
	}
}

func TestParseWAMessage_Food(t *testing.T) {
	cases := []string{
		"makan nasi goreng",
		"sarapan bubur ayam",
		"makan siang gado-gado",
		"makan malam soto",
		"snack keripik",
	}
	for _, c := range cases {
		got := ParseWAMessage(c)
		if got.Err != nil {
			t.Errorf("%q: unexpected error: %v", c, got.Err)
			continue
		}
		if got.MetricType != "food" {
			t.Errorf("%q: want metric food, got %s", c, got.MetricType)
		}
		if got.ValueText == nil || *got.ValueText == "" {
			t.Errorf("%q: want non-empty value_text", c)
		}
	}
}

func TestParseWAMessage_Unknown(t *testing.T) {
	cases := []string{
		"halo",
		"apa kabar",
		"terima kasih",
		"ok",
	}
	for _, c := range cases {
		got := ParseWAMessage(c)
		if got.Err == nil {
			t.Errorf("%q: expected error for unknown message, got metric=%s", c, got.MetricType)
		}
	}
}

func TestParseWAMessage_BPInvalidValues(t *testing.T) {
	got := ParseWAMessage("tensi 80/120") // diastolic > systolic
	if got.Err == nil {
		t.Error("expected error for reversed BP values")
	}
}
