// Package report merender Pre-Visit Brief menjadi laporan HTML siap-cetak (Print→PDF).
// Layer delivery: html/template stdlib + inline SVG (tanpa dependensi/aset eksternal),
// tidak menyentuh DB — hanya memformat model.BriefReportData yang sudah disusun usecase.
package report

import (
	"bytes"
	_ "embed"
	"fmt"
	"html/template"
	"strings"
	"time"

	"sehatiku-backend/internal/model"
)

//go:embed brief_report.html
var briefReportHTML string

var wibLocation = mustLoadWIB()

func mustLoadWIB() *time.Location {
	loc, err := time.LoadLocation("Asia/Jakarta")
	if err != nil {
		return time.FixedZone("WIB", 7*3600) // ponytail: fallback bila tzdata absen di host
	}
	return loc
}

var briefTemplate = template.Must(
	template.New("brief_report").Funcs(template.FuncMap{
		"wibDateTime":    wibDateTime,
		"wibDateTimePtr": wibDateTimePtr,
		"sexLabel":       sexLabel,
		"diseaseLabel":   diseaseLabel,
		"tierLabel":      tierLabel,
		"statusLabel":    statusLabel,
		"feedbackLabel":  feedbackLabel,
		"f1":             func(v float64) string { return fmt.Sprintf("%.1f", v) },
		"slopeText":      slopeText,
		"intp": func(v *int) string {
			if v == nil {
				return "—"
			}
			return fmt.Sprintf("%d", *v)
		},
	}).Parse(briefReportHTML),
)

// reportView adalah model tampilan datar: brief mentah + kop pasien + SVG terkomputasi.
type reportView struct {
	Patient        model.BriefPatientHeader
	B              *model.PreVisitBriefResponse
	RiskColor      string
	GlucoseChart   template.HTML
	BPChart        template.HTML
	WeightChart    template.HTML
	AdherenceChart template.HTML
}

// RenderBrief mengeksekusi template dan mengembalikan HTML laporan.
func RenderBrief(d *model.BriefReportData) (string, error) {
	daily := d.Brief.Trajectory.Daily
	view := reportView{
		Patient:        d.Patient,
		B:              d.Brief,
		RiskColor:      riskColor(d.Brief.Risk),
		GlucoseChart:   lineChartSVG([]chartSeries{{vals: glucoseSeries(daily), color: "#2563eb"}}, " mg/dL"),
		BPChart:        lineChartSVG([]chartSeries{{vals: bpSeries(daily, true), color: "#dc2626"}, {vals: bpSeries(daily, false), color: "#f59e0b"}}, " mmHg"),
		WeightChart:    lineChartSVG([]chartSeries{{vals: weightSeries(daily), color: "#059669"}}, " kg"),
		AdherenceChart: barChartSVG(d.Brief.MedAdherence.MissedWeekdays),
	}

	var buf bytes.Buffer
	if err := briefTemplate.Execute(&buf, view); err != nil {
		return "", fmt.Errorf("render brief report: %w", err)
	}
	return buf.String(), nil
}

// --- Label & format helpers (dipakai template FuncMap) ---

func wibDateTime(t time.Time) string { return t.In(wibLocation).Format("02 Jan 2006 15:04 WIB") }

func wibDateTimePtr(t *time.Time) string {
	if t == nil {
		return "—"
	}
	return wibDateTime(*t)
}

func sexLabel(s string) string {
	switch s {
	case "male":
		return "Laki-laki"
	case "female":
		return "Perempuan"
	default:
		return "—"
	}
}

func diseaseLabel(s string) string {
	switch s {
	case "diabetes_t2":
		return "Diabetes Tipe 2"
	case "hypertension":
		return "Hipertensi"
	case "both":
		return "Diabetes + Hipertensi"
	default:
		return s
	}
}

func tierLabel(s string) string {
	switch s {
	case "acute_today":
		return "Akut (hari ini)"
	case "trend_this_week":
		return "Tren (minggu ini)"
	default:
		return s
	}
}

func statusLabel(s string) string {
	if s == "" {
		return "—"
	}
	return strings.ToUpper(s[:1]) + s[1:]
}

func feedbackLabel(s *string) string {
	if s == nil {
		return "Belum dinilai"
	}
	switch *s {
	case "accurate":
		return "Akurat"
	case "inaccurate":
		return "Tidak akurat"
	default:
		return *s
	}
}

func slopeText(v *float64, unit string) string {
	if v == nil {
		return "tren belum bisa dihitung (data < 3 titik)"
	}
	sign := "+"
	if *v < 0 {
		sign = ""
	}
	return fmt.Sprintf("%s%.1f%s / minggu", sign, *v, unit)
}

// riskColor memetakan status risiko ke warna aksen laporan (aman/waswas/bahaya).
func riskColor(r *model.BriefRisk) string {
	if r == nil {
		return "#6b7280"
	}
	switch r.Status {
	case "aman":
		return "#059669"
	case "waswas":
		return "#f59e0b"
	case "bahaya":
		return "#dc2626"
	default:
		return "#6b7280"
	}
}

// --- SVG charts (dibangun dari angka saja → aman ditandai template.HTML) ---

type chartSeries struct {
	vals  []*float64
	color string
}

func glucoseSeries(daily []model.RecordHistoryItem) []*float64 {
	out := make([]*float64, len(daily))
	for i, d := range daily {
		out[i] = d.BloodSugar
	}
	return out
}

func weightSeries(daily []model.RecordHistoryItem) []*float64 {
	out := make([]*float64, len(daily))
	for i, d := range daily {
		out[i] = d.Weight
	}
	return out
}

// bpSeries: systolic bila sys=true, else diastolic (int → *float64, nil = tak ada).
func bpSeries(daily []model.RecordHistoryItem, sys bool) []*float64 {
	out := make([]*float64, len(daily))
	for i, d := range daily {
		src := d.Diastolic
		if sys {
			src = d.Systolic
		}
		if src != nil {
			v := float64(*src)
			out[i] = &v
		}
	}
	return out
}

const (
	chartW    = 540
	chartH    = 170
	padLeft   = 40
	padRight  = 12
	padTop    = 12
	padBottom = 24
)

// lineChartSVG menggambar satu/lebih deret garis (nil = putus). Kosong / semua-nil → placeholder.
func lineChartSVG(series []chartSeries, unit string) template.HTML {
	min, max, n, any := seriesRange(series)
	if !any || n < 2 {
		return placeholderSVG()
	}
	plotW := float64(chartW - padLeft - padRight)
	plotH := float64(chartH - padTop - padBottom)
	xAt := func(i int) float64 { return padLeft + float64(i)/float64(n-1)*plotW }
	yAt := func(v float64) float64 {
		if max == min {
			return padTop + plotH/2
		}
		return padTop + (1-(v-min)/(max-min))*plotH
	}

	var b strings.Builder
	fmt.Fprintf(&b, `<svg viewBox="0 0 %d %d" width="100%%" preserveAspectRatio="xMidYMid meet" role="img">`, chartW, chartH)
	b.WriteString(`<rect x="0" y="0" width="100%" height="100%" fill="#fafafa"/>`)
	// gridlines + label sumbu-y (min, tengah, max)
	for _, gv := range []float64{min, (min + max) / 2, max} {
		y := yAt(gv)
		fmt.Fprintf(&b, `<line x1="%d" y1="%.1f" x2="%d" y2="%.1f" stroke="#e5e7eb" stroke-width="1"/>`, padLeft, y, chartW-padRight, y)
		fmt.Fprintf(&b, `<text x="%d" y="%.1f" font-size="9" fill="#9ca3af" text-anchor="end">%.0f</text>`, padLeft-4, y+3, gv)
	}
	for _, s := range series {
		b.WriteString(polyline(s, xAt, yAt))
	}
	fmt.Fprintf(&b, `<text x="%d" y="%d" font-size="9" fill="#9ca3af">%s</text>`, padLeft, chartH-6, strings.TrimSpace(unit))
	b.WriteString(`</svg>`)
	return template.HTML(b.String()) // #nosec G203 — angka + konstanta internal, tanpa input user
}

// polyline: segmen garis + titik, putus di nil.
func polyline(s chartSeries, xAt func(int) float64, yAt func(float64) float64) string {
	var b strings.Builder
	pen := false
	for i, v := range s.vals {
		if v == nil {
			pen = false
			continue
		}
		x, y := xAt(i), yAt(*v)
		if !pen {
			fmt.Fprintf(&b, `<path fill="none" stroke="%s" stroke-width="2" d="M %.1f %.1f`, s.color, x, y)
			pen = true
		} else {
			fmt.Fprintf(&b, ` L %.1f %.1f`, x, y)
		}
		// tutup path bila titik berikutnya nil / habis
		last := i == len(s.vals)-1
		next := !last && s.vals[i+1] != nil
		if !next {
			b.WriteString(`"/>`)
			pen = false
		}
	}
	for i, v := range s.vals {
		if v == nil {
			continue
		}
		fmt.Fprintf(&b, `<circle cx="%.1f" cy="%.1f" r="2.4" fill="%s"/>`, xAt(i), yAt(*v), s.color)
	}
	return b.String()
}

func seriesRange(series []chartSeries) (min, max float64, n int, any bool) {
	for _, s := range series {
		if len(s.vals) > n {
			n = len(s.vals)
		}
		for _, v := range s.vals {
			if v == nil {
				continue
			}
			if !any {
				min, max, any = *v, *v, true
				continue
			}
			if *v < min {
				min = *v
			}
			if *v > max {
				max = *v
			}
		}
	}
	return
}

// barChartSVG: batang lupa-obat per hari (Senin..Minggu). Kosong → placeholder.
func barChartSVG(missed map[string]int) template.HTML {
	order := []string{"Senin", "Selasa", "Rabu", "Kamis", "Jumat", "Sabtu", "Minggu"}
	maxV := 0
	for _, d := range order {
		if missed[d] > maxV {
			maxV = missed[d]
		}
	}
	if maxV == 0 {
		return placeholderSVG()
	}
	plotH := float64(chartH - padTop - padBottom)
	slot := float64(chartW-padLeft-padRight) / float64(len(order))
	barW := slot * 0.6

	var b strings.Builder
	fmt.Fprintf(&b, `<svg viewBox="0 0 %d %d" width="100%%" preserveAspectRatio="xMidYMid meet" role="img">`, chartW, chartH)
	b.WriteString(`<rect x="0" y="0" width="100%" height="100%" fill="#fafafa"/>`)
	for i, d := range order {
		v := missed[d]
		h := float64(v) / float64(maxV) * plotH
		x := padLeft + float64(i)*slot + (slot-barW)/2
		y := float64(padTop) + plotH - h
		if v > 0 {
			fmt.Fprintf(&b, `<rect x="%.1f" y="%.1f" width="%.1f" height="%.1f" fill="#dc2626" rx="2"/>`, x, y, barW, h)
			fmt.Fprintf(&b, `<text x="%.1f" y="%.1f" font-size="9" fill="#6b7280" text-anchor="middle">%d</text>`, x+barW/2, y-3, v)
		}
		fmt.Fprintf(&b, `<text x="%.1f" y="%d" font-size="9" fill="#6b7280" text-anchor="middle">%s</text>`, x+barW/2, chartH-8, d[:3])
	}
	b.WriteString(`</svg>`)
	return template.HTML(b.String()) // #nosec G203 — angka + konstanta internal
}

func placeholderSVG() template.HTML {
	return template.HTML(fmt.Sprintf(
		`<svg viewBox="0 0 %d %d" width="100%%" preserveAspectRatio="xMidYMid meet" role="img"><rect width="100%%" height="100%%" fill="#fafafa"/><text x="50%%" y="50%%" font-size="12" fill="#9ca3af" text-anchor="middle">data belum cukup</text></svg>`,
		chartW, chartH))
}
