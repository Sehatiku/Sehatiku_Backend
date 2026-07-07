package helper

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"unicode"
)

// ParsedMetric adalah hasil parse satu pesan WhatsApp pasien/pendamping.
// Tepat satu dari ValueNumeric, BPSystolic+BPDiastolic, atau ValueText akan terisi
// sesuai MetricType. Err != nil berarti pesan tidak bisa diparsing.
type ParsedMetric struct {
	MetricType   string
	ValueNumeric *float64
	BPSystolic   *int
	BPDiastolic  *int
	ValueText    *string
	Err          error
}

// ParseWAMessage mencoba mengidentifikasi satu metrik kesehatan dari pesan teks bebas
// berbahasa Indonesia. Rule-based (regex + keyword matching) — cukup untuk MVP dan mudah
// diperluas. Mengembalikan Err != nil bila pesan tidak dikenali atau nilainya tidak valid.
//
// Format yang didukung (case-insensitive):
//   - Gula darah  : "gula 180", "gula darah 140 mg/dl", "gds 180"
//   - Tekanan darah: "tensi 120/80", "td 130/85", "tekanan darah 120/80"
//   - Kepatuhan   : "obat ya", "minum obat", "obat tidak", "tidak minum obat"
//   - Makanan     : "makan nasi goreng", "sarapan bubur ayam", "makan siang gado-gado"
//   - Olahraga    : "olahraga 30 menit", "jalan 45", "lari 20 menit"
//   - Tidur       : "tidur 7 jam", "tidur 7.5 jam"
//   - Stres       : "stres 3", "stress 5"
//   - Berat badan : "berat 65 kg", "bb 70", "berat badan 68"
//   - Rokok       : "rokok 5 batang", "merokok 3", "tidak rokok", "berhenti rokok"
//   - Alkohol     : "alkohol 2 unit", "minum alkohol 1"
func ParseWAMessage(text string) ParsedMetric {
	normalized := strings.TrimSpace(strings.ToLower(text))
	// hapus tanda baca berlebih di awal/akhir, ganti tab/newline dengan spasi
	normalized = strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return ' '
		}
		return r
	}, normalized)
	normalized = strings.TrimSpace(normalized)

	// Coba setiap parser secara berurutan. Urutan penting — bp sebelum numerik umum.
	parsers := []func(string) (ParsedMetric, bool){
		parseBP,
		parseGlucose,
		parseMedAdherence,
		parseActivity,
		parseSleep,
		parseStress,
		parseWeight,
		parseSmoking,
		parseAlcohol,
		parseFood, // paling terakhir: catchall teks makanan
	}
	for _, p := range parsers {
		if m, ok := p(normalized); ok {
			return m
		}
	}

	return ParsedMetric{
		Err: fmt.Errorf("pesan tidak dikenali: %q — gunakan format yang benar (contoh: gula 180, tensi 120/80, obat ya)", text),
	}
}

// ─── helpers ────────────────────────────────────────────────────────────────

// numRE mencocokkan angka desimal (titik atau koma sebagai pemisah) dalam teks.
var numRE = regexp.MustCompile(`(\d+[.,]?\d*)`)

func extractNumber(s string) (float64, bool) {
	m := numRE.FindString(s)
	if m == "" {
		return 0, false
	}
	m = strings.ReplaceAll(m, ",", ".")
	v, err := strconv.ParseFloat(m, 64)
	if err != nil {
		return 0, false
	}
	return v, true
}

func ptr[T any](v T) *T { return &v }

// ─── parsers ────────────────────────────────────────────────────────────────

// bpRE cocok: "120/80", "120 / 80", "120-80"
var bpRE = regexp.MustCompile(`(\d{2,3})\s*[/\-]\s*(\d{2,3})`)

func parseBP(s string) (ParsedMetric, bool) {
	keywords := []string{"tensi", "tekanan darah", "td ", "bp ", "sistolik"}
	hasKW := false
	for _, kw := range keywords {
		if strings.Contains(s, kw) {
			hasKW = true
			break
		}
	}
	// Juga cocok bila ada pola angka/angka tanpa keyword (sangat khas BP)
	m := bpRE.FindStringSubmatch(s)
	if m == nil {
		return ParsedMetric{}, false
	}
	if !hasKW && !bpLookingLikeOnly(s) {
		// Ada slash/dash tapi mungkin bukan BP (misal "olahraga 30-45 menit") — skip
		return ParsedMetric{}, false
	}
	sys, _ := strconv.Atoi(m[1])
	dia, _ := strconv.Atoi(m[2])
	if sys < 40 || sys > 300 || dia < 20 || dia > 200 || sys <= dia {
		return ParsedMetric{Err: fmt.Errorf("nilai tekanan darah tidak valid (sistolik %d, diastolik %d)", sys, dia)}, true
	}
	return ParsedMetric{MetricType: "bp", BPSystolic: ptr(sys), BPDiastolic: ptr(dia)}, true
}

// bpLookingLikeOnly: heuristik — string hampir seluruhnya adalah pola NNN/NNN ± sedikit teks
func bpLookingLikeOnly(s string) bool {
	// hapus pola bp sendiri, sisa tidak lebih dari 30 karakter
	cleaned := bpRE.ReplaceAllString(s, "")
	return len(strings.TrimSpace(cleaned)) <= 30
}

func parseGlucose(s string) (ParsedMetric, bool) {
	keywords := []string{"gula", "glukosa", "gds", "gdp", "gula darah", "kadar gula"}
	for _, kw := range keywords {
		if strings.Contains(s, kw) {
			v, ok := extractNumber(s)
			if !ok {
				return ParsedMetric{Err: fmt.Errorf("angka gula darah tidak ditemukan dalam pesan")}, true
			}
			return ParsedMetric{MetricType: "glucose", ValueNumeric: ptr(v)}, true
		}
	}
	return ParsedMetric{}, false
}

// medYesRE: pola "obat ya", "minum obat", "sudah minum obat", "pakai obat"
var medYesRE = regexp.MustCompile(`(minum|sudah|pakai|konsumsi)\s+obat|obat\s+(ya|iya|sudah|diminum|dimakan|ok|✓|👍)`)
var medNoRE = regexp.MustCompile(`(tidak|ga|gak|belum|lupa)\s+(minum\s+)?obat|obat\s+(tidak|ga|gak|belum|lupa|tidak\s+diminum)`)

func parseMedAdherence(s string) (ParsedMetric, bool) {
	if strings.Contains(s, "obat") {
		if medNoRE.MatchString(s) {
			return ParsedMetric{MetricType: "med_adherence", ValueNumeric: ptr(0.0)}, true
		}
		if medYesRE.MatchString(s) || strings.Contains(s, "obat") {
			// "obat ya" / "minum obat" / plain "obat" dianggap sudah minum
			return ParsedMetric{MetricType: "med_adherence", ValueNumeric: ptr(100.0)}, true
		}
	}
	return ParsedMetric{}, false
}

func parseActivity(s string) (ParsedMetric, bool) {
	keywords := []string{"olahraga", "jalan", "lari", "senam", "bersepeda", "sepeda", "renang", "jogging", "gym", "aktivitas"}
	for _, kw := range keywords {
		if strings.Contains(s, kw) {
			v, ok := extractNumber(s)
			if !ok {
				// "olahraga" tanpa angka → default 30 menit sebagai fallback minimal
				return ParsedMetric{MetricType: "activity", ValueNumeric: ptr(30.0)}, true
			}
			return ParsedMetric{MetricType: "activity", ValueNumeric: ptr(v)}, true
		}
	}
	return ParsedMetric{}, false
}

func parseSleep(s string) (ParsedMetric, bool) {
	if strings.Contains(s, "tidur") {
		v, ok := extractNumber(s)
		if !ok {
			return ParsedMetric{Err: fmt.Errorf("durasi tidur tidak ditemukan dalam pesan")}, true
		}
		return ParsedMetric{MetricType: "sleep", ValueNumeric: ptr(v)}, true
	}
	return ParsedMetric{}, false
}

func parseStress(s string) (ParsedMetric, bool) {
	if strings.Contains(s, "stres") || strings.Contains(s, "stress") {
		v, ok := extractNumber(s)
		if !ok {
			return ParsedMetric{Err: fmt.Errorf("level stres (1-10) tidak ditemukan dalam pesan")}, true
		}
		return ParsedMetric{MetricType: "stress", ValueNumeric: ptr(v)}, true
	}
	return ParsedMetric{}, false
}

func parseWeight(s string) (ParsedMetric, bool) {
	keywords := []string{"berat badan", "berat", "bb ", "bb\t", "timbang"}
	for _, kw := range keywords {
		if strings.Contains(s, kw) {
			v, ok := extractNumber(s)
			if !ok {
				return ParsedMetric{Err: fmt.Errorf("angka berat badan tidak ditemukan dalam pesan")}, true
			}
			return ParsedMetric{MetricType: "weight", ValueNumeric: ptr(v)}, true
		}
	}
	return ParsedMetric{}, false
}

var smokeNoRE = regexp.MustCompile(`(tidak|ga|gak|berhenti|stop|quit|sudah\s+berhenti)\s+(rokok|merokok)`)

func parseSmoking(s string) (ParsedMetric, bool) {
	if strings.Contains(s, "rokok") || strings.Contains(s, "merokok") {
		if smokeNoRE.MatchString(s) {
			return ParsedMetric{MetricType: "smoking", ValueNumeric: ptr(0.0)}, true
		}
		v, ok := extractNumber(s)
		if !ok {
			// "rokok" tanpa angka → asumsikan 1 batang
			return ParsedMetric{MetricType: "smoking", ValueNumeric: ptr(1.0)}, true
		}
		return ParsedMetric{MetricType: "smoking", ValueNumeric: ptr(v)}, true
	}
	return ParsedMetric{}, false
}

func parseAlcohol(s string) (ParsedMetric, bool) {
	if strings.Contains(s, "alkohol") || strings.Contains(s, "miras") || strings.Contains(s, "minuman keras") {
		v, ok := extractNumber(s)
		if !ok {
			return ParsedMetric{MetricType: "alcohol", ValueNumeric: ptr(1.0)}, true
		}
		return ParsedMetric{MetricType: "alcohol", ValueNumeric: ptr(v)}, true
	}
	return ParsedMetric{}, false
}

// foodKeywords adalah awalan pesan yang mengindikasikan laporan makanan/minuman.
var foodKeywords = []string{
	"makan", "sarapan", "makan siang", "makan malam", "snack", "camilan",
	"cemilan", "jajan", "minum", "makanan", "minuman", "konsumsi",
}

func parseFood(s string) (ParsedMetric, bool) {
	for _, kw := range foodKeywords {
		if strings.HasPrefix(s, kw) || strings.Contains(s, kw) {
			// Kembalikan teks asli (case-insensitive sudah dinormalisasi) sebagai value_text
			// Panjang maks 500 char ditegakkan usecase, bukan parser.
			return ParsedMetric{MetricType: "food", ValueText: ptr(s)}, true
		}
	}
	return ParsedMetric{}, false
}

// ─── template log harian (form multi-metrik) ──────────────────────────────────

// logTemplateTriggers adalah frasa yang menandakan pengirim ingin TEMPLATE pengisian
// log harian (mis. "saya ingin tulis log harian"), bukan sedang mencatat sebuah metrik.
// Dicocokkan sebagai substring pada teks lowercase — sengaja spesifik agar tidak
// bentrok dengan pesan pencatatan biasa (mis. "menu makan siang" bukan trigger).
var logTemplateTriggers = []string{
	"tulis log", "isi log", "log harian", "catat harian", "lapor harian",
	"mau lapor", "ingin lapor", "mulai lapor", "template", "format log", "cara lapor",
}

// IsLogTemplateRequest mengembalikan true bila pesan meminta template log harian.
// Caller membalas dengan form kosong untuk diisi, bukan mem-parsing metrik.
func IsLogTemplateRequest(text string) bool {
	s := strings.ToLower(strings.TrimSpace(text))
	for _, kw := range logTemplateTriggers {
		if strings.Contains(s, kw) {
			return true
		}
	}
	return false
}

// formLabelMetric memetakan label kolom template (bagian kiri titik dua) ke metric_type.
// HasPrefix dipakai agar "gula darah" tetap kena label "gula". Entri lebih spesifik
// didahulukan supaya tidak keburu kena label pendek yang jadi prefix-nya.
var formLabelMetric = []struct{ label, metric string }{
	{"tekanan darah", "bp"}, {"gula darah", "glucose"}, {"berat badan", "weight"},
	{"tensi", "bp"}, {"td", "bp"},
	{"gula", "glucose"}, {"gds", "glucose"}, {"gdp", "glucose"},
	{"obat", "med_adherence"},
	{"olahraga", "activity"}, {"aktivitas", "activity"}, {"jalan", "activity"},
	{"tidur", "sleep"},
	{"stres", "stress"}, {"stress", "stress"},
	{"berat", "weight"}, {"bb", "weight"}, {"timbang", "weight"},
	{"rokok", "smoking"}, {"alkohol", "alcohol"},
	{"makanan", "food"}, {"makan", "food"},
}

func formMetricFor(label string) string {
	for _, e := range formLabelMetric {
		if strings.HasPrefix(label, e.label) {
			return e.metric
		}
	}
	return ""
}

// ParseLogForm mem-parsing pesan template multi-baris "Label: nilai" menjadi beberapa
// metrik. Baris kosong, baris placeholder (mengandung "_"), baris tanpa titik dua/
// sama-dengan, dan label tak dikenal dilewati diam-diam — sehingga template yang
// diisi sebagian tetap tercatat baris terisinya saja. Mengembalikan daftar metrik
// valid; kosong bila tak ada satu pun baris form terisi (caller fallback ke parser
// teks-bebas satu-metrik).
func ParseLogForm(text string) []ParsedMetric {
	var out []ParsedMetric
	for raw := range strings.SplitSeq(text, "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.Contains(line, "_") {
			continue
		}
		idx := strings.IndexAny(line, ":=")
		if idx < 0 {
			continue
		}
		label := strings.TrimSpace(strings.ToLower(line[:idx]))
		value := strings.TrimSpace(line[idx+1:])
		if label == "" || value == "" {
			continue
		}
		metric := formMetricFor(label)
		if metric == "" {
			continue
		}
		if m, ok := parseFormValue(metric, value); ok {
			out = append(out, m)
		}
	}
	return out
}

// parseFormValue mengubah satu nilai kolom template menjadi ParsedMetric sesuai metric.
// Berbeda dari parser teks-bebas: label sudah menentukan metric, jadi kita hanya
// mengekstrak nilai — menghindari ambiguitas (mis. "obat: tidak" harus 0, bukan 100).
func parseFormValue(metric, value string) (ParsedMetric, bool) {
	v := strings.ToLower(value)
	switch metric {
	case "bp":
		m := bpRE.FindStringSubmatch(v)
		if m == nil {
			return ParsedMetric{}, false
		}
		sys, _ := strconv.Atoi(m[1])
		dia, _ := strconv.Atoi(m[2])
		if sys < 40 || sys > 300 || dia < 20 || dia > 200 || sys <= dia {
			return ParsedMetric{}, false
		}
		return ParsedMetric{MetricType: "bp", BPSystolic: ptr(sys), BPDiastolic: ptr(dia)}, true
	case "med_adherence":
		neg := v == "ga" || v == "gak" || v == "no" || v == "0" ||
			strings.Contains(v, "tidak") || strings.Contains(v, "tdk") ||
			strings.Contains(v, "belum") || strings.Contains(v, "lupa") || strings.Contains(v, "nggak")
		if neg {
			return ParsedMetric{MetricType: "med_adherence", ValueNumeric: ptr(0.0)}, true
		}
		return ParsedMetric{MetricType: "med_adherence", ValueNumeric: ptr(100.0)}, true
	case "food":
		return ParsedMetric{MetricType: "food", ValueText: ptr(v)}, true
	default: // metrik numerik: glucose, weight, sleep, activity, stress, smoking, alcohol
		n, ok := extractNumber(v)
		if !ok {
			return ParsedMetric{}, false
		}
		return ParsedMetric{MetricType: metric, ValueNumeric: ptr(n)}, true
	}
}
