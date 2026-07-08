// Package gemini is the HTTP gateway to Google's Gemini generative API
// (generativelanguage.googleapis.com). It mirrors the structure of
// internal/gateway/ml (MLGateway): a small typed client with a constructor,
// typed errors, and a status-code switch.
//
// Dipakai oleh usecase summary untuk menyusun narasi ringkasan kesehatan dari
// angka agregat yang dihitung backend (lihat internal/usecase/summary_usecase.go).
//
// Wiring (di internal/config/app.go BootStrap):
//
//	geminigw "sehatiku-backend/internal/gateway/gemini"
//	...
//	geminiGateway := geminigw.New(
//		config.Config.GetString("GEMINI_API_KEY"),
//		config.Config.GetString("GEMINI_MODEL"), // kosong -> default gemini-2.5-flash
//		config.Log,
//	)
//
// Env vars (.env / .env.example):
//
//	GEMINI_API_KEY=<api key dari Google AI Studio>
//	GEMINI_MODEL=gemini-2.5-flash
package gemini

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"sehatiku-backend/internal/model"

	"go.uber.org/zap"
)

var (
	ErrGeminiBadRequest   = errors.New("permintaan tidak valid untuk Gemini API")
	ErrGeminiUnauthorized = errors.New("Gemini API key tidak valid")
	ErrGeminiUpstream     = errors.New("layanan Gemini tidak tersedia sementara")
	ErrGeminiEmpty        = errors.New("Gemini tidak mengembalikan teks")
)

const (
	defaultModel = "gemini-2.5-flash"
	baseURL      = "https://generativelanguage.googleapis.com/v1beta/models"
)

// GeminiGateway is an HTTP client for the Gemini generateContent endpoint.
type GeminiGateway struct {
	APIKey     string
	Model      string
	HTTPClient *http.Client
	Log        *zap.Logger
}

// New builds a gateway. model kosong -> default gemini-2.5-flash. apiKey kosong
// diperbolehkan saat boot (gateway tetap dibangun), tapi panggilan akan gagal
// dengan ErrGeminiUnauthorized hingga key diisi.
func New(apiKey, model string, log *zap.Logger) *GeminiGateway {
	if model == "" {
		model = defaultModel
	}
	return &GeminiGateway{
		APIKey: apiKey,
		Model:  model,
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		Log: log,
	}
}

// --- request/response shapes (subset of the Gemini REST API) ---

type genInlineData struct {
	MimeType string `json:"mime_type"`
	Data     string `json:"data"` // base64-encoded bytes
}

type genPart struct {
	Text       string         `json:"text,omitempty"`
	InlineData *genInlineData `json:"inline_data,omitempty"`
}

type genContent struct {
	Parts []genPart `json:"parts"`
}

// thinkingConfig menonaktifkan "thinking" pada model Gemini 2.5 (thinkingBudget=0)
// agar seluruh anggaran token dipakai untuk jawaban, bukan reasoning internal —
// tanpa ini model 2.5-flash bisa menghabiskan token pada thoughts dan mengembalikan
// kandidat tanpa teks (finishReason MAX_TOKENS).
type thinkingConfig struct {
	ThinkingBudget int `json:"thinkingBudget"`
}

type genConfig struct {
	Temperature      float64         `json:"temperature"`
	MaxOutputTokens  int             `json:"maxOutputTokens"`
	ThinkingConfig   *thinkingConfig `json:"thinkingConfig,omitempty"`
	ResponseMIMEType string          `json:"responseMimeType,omitempty"`
}

type generateRequest struct {
	Contents         []genContent `json:"contents"`
	GenerationConfig genConfig    `json:"generationConfig"`
}

type generateResponse struct {
	Candidates []struct {
		Content struct {
			Parts []genPart `json:"parts"`
		} `json:"content"`
		FinishReason string `json:"finishReason"`
	} `json:"candidates"`
}

// GenerateSummary mengirim prompt ke Gemini dan mengembalikan teks narasi tunggal.
func (g *GeminiGateway) GenerateSummary(ctx context.Context, prompt string) (string, error) {
	cfg := genConfig{
		Temperature:     0.4,
		MaxOutputTokens: 1024,
	}
	g.applyThinking(&cfg)

	reqBody := generateRequest{
		Contents:         []genContent{{Parts: []genPart{{Text: prompt}}}},
		GenerationConfig: cfg,
	}
	return g.doGenerate(ctx, reqBody)
}

// applyThinking mematikan "thinking" hanya untuk model 2.5 (lihat thinkingConfig). Mengirim
// thinkingBudget ke model lain (mis. 2.0-flash) bisa ditolak 400.
func (g *GeminiGateway) applyThinking(cfg *genConfig) {
	if strings.Contains(g.Model, "2.5") {
		cfg.ThinkingConfig = &thinkingConfig{ThinkingBudget: 0}
	}
}

// doGenerate mengeksekusi satu panggilan generateContent dan mengembalikan teks kandidat
// tergabung. Dipakai bersama oleh GenerateSummary (narasi) dan ExtractBaseline (JSON).
func (g *GeminiGateway) doGenerate(ctx context.Context, reqBody generateRequest) (string, error) {
	if g.APIKey == "" {
		return "", ErrGeminiUnauthorized
	}

	payload, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("encoding Gemini request: %w", err)
	}

	endpoint := fmt.Sprintf("%s/%s:generateContent?key=%s", baseURL, g.Model, url.QueryEscape(g.APIKey))
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, bytes.NewReader(payload))
	if err != nil {
		return "", fmt.Errorf("creating Gemini request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := g.HTTPClient.Do(req)
	if err != nil {
		g.Log.Warn("Gemini API unreachable", zap.Error(err))
		return "", fmt.Errorf("%w: %v", ErrGeminiUpstream, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("reading Gemini response: %w", err)
	}

	switch resp.StatusCode {
	case http.StatusOK:
		// handled below
	case http.StatusUnauthorized, http.StatusForbidden:
		g.Log.Error("Gemini API key rejected", zap.Int("status", resp.StatusCode))
		return "", ErrGeminiUnauthorized
	case http.StatusBadRequest:
		g.Log.Warn("Gemini upstream: bad request", zap.String("body", string(body)))
		return "", fmt.Errorf("%w: %s", ErrGeminiBadRequest, string(body))
	default:
		g.Log.Warn("unexpected Gemini API status", zap.Int("status", resp.StatusCode), zap.String("body", string(body)))
		return "", fmt.Errorf("%w: status %d", ErrGeminiUpstream, resp.StatusCode)
	}

	var out generateResponse
	if err := json.Unmarshal(body, &out); err != nil {
		return "", fmt.Errorf("parsing Gemini response: %w", err)
	}

	if len(out.Candidates) == 0 || len(out.Candidates[0].Content.Parts) == 0 {
		return "", ErrGeminiEmpty
	}

	var sb strings.Builder
	for _, p := range out.Candidates[0].Content.Parts {
		sb.WriteString(p.Text)
	}
	text := strings.TrimSpace(sb.String())
	if text == "" {
		return "", ErrGeminiEmpty
	}
	return text, nil
}

// baselineExtractPrompt menginstruksikan Gemini membaca dokumen template baseline Sehatiku
// yang sudah diisi faskes dan mengembalikan JSON ketat. Hanya field yang diisi faskes yang
// diminta — field turunan (bmi_category, hypertension_status, dll.) dihitung backend, jadi
// TIDAK diminta di sini. Nilai yang tidak terbaca dikosongkan (0 / null), tidak dikarang.
const baselineExtractPrompt = `Anda mengekstrak data dari dokumen "Template Baseline Klinis Sehatiku" yang sudah diisi (hasil scan/foto).
Kembalikan HANYA objek JSON valid (tanpa markdown, tanpa penjelasan) dengan field berikut:
{
  "bmi": number,                       // Indeks Massa Tubuh
  "waist_circumference_cm": number,    // lingkar pinggang (cm)
  "smoking_status": "never|former|current",
  "alcohol_use": boolean,
  "physical_activity": "sedentary|light|moderate|active",
  "family_history_diabetes": boolean,
  "family_history_cvd": boolean,
  "systolic_bp_mmhg": integer,
  "diastolic_bp_mmhg": integer,
  "fasting_glucose_mgdl": number,      // gula darah puasa
  "hba1c_pct": number,
  "total_cholesterol_mgdl": number,
  "hdl_mgdl": number,
  "ldl_mgdl": number,
  "triglycerides_mgdl": number,
  "cvd_risk_10yr_pct": number,         // opsional; 0 bila tidak tercantum
  "cvd_risk_category": "low|moderate|high|very_high",  // opsional; "" bila tidak tercantum
  "on_antihypertensive": boolean,
  "on_antidiabetic": boolean,
  "on_statin": boolean,
  "target_risk": "low|medium|high",    // Rendah=low, Menengah=medium, Tinggi=high
  "egfr": number,
  "uacr": number,                      // opsional; 0 bila tidak tercantum
  "diagnosis": "diabetes|hipertensi|komplikasi"  // opsional; "" bila tidak tercantum
}
Aturan: gunakan satuan seperti tertulis di dokumen; untuk checkbox/centang, true bila dicentang "Ya"/ada tanda, false bila "Tidak"/kosong; JANGAN mengarang nilai yang tidak ada—isi 0 untuk angka atau "" untuk string.`

// ExtractBaseline mengirim dokumen (gambar/PDF) template baseline terisi ke Gemini vision dan
// mengembalikan field baseline yang diisi faskes untuk mem-prefill form registrasi. Ini HANYA
// prefill; faskes tetap meninjau/mengoreksi sebelum submit.
func (g *GeminiGateway) ExtractBaseline(ctx context.Context, fileBytes []byte, mimeType string) (*model.PatientBaselineRequest, error) {
	cfg := genConfig{
		Temperature:      0.0, // ekstraksi deterministik, bukan kreatif
		MaxOutputTokens:  2048,
		ResponseMIMEType: "application/json",
	}
	g.applyThinking(&cfg)

	reqBody := generateRequest{
		Contents: []genContent{{Parts: []genPart{
			{Text: baselineExtractPrompt},
			{InlineData: &genInlineData{MimeType: mimeType, Data: base64.StdEncoding.EncodeToString(fileBytes)}},
		}}},
		GenerationConfig: cfg,
	}

	text, err := g.doGenerate(ctx, reqBody)
	if err != nil {
		return nil, err
	}

	var out model.PatientBaselineRequest
	if err := json.Unmarshal([]byte(text), &out); err != nil {
		g.Log.Warn("Gemini baseline extraction: invalid JSON", zap.String("body", text), zap.Error(err))
		return nil, fmt.Errorf("%w: ekstraksi baseline tidak menghasilkan JSON valid", ErrGeminiBadRequest)
	}
	return &out, nil
}
