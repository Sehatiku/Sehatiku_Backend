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
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

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

type genPart struct {
	Text string `json:"text"`
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
	Temperature     float64         `json:"temperature"`
	MaxOutputTokens int             `json:"maxOutputTokens"`
	ThinkingConfig  *thinkingConfig `json:"thinkingConfig,omitempty"`
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
	if g.APIKey == "" {
		return "", ErrGeminiUnauthorized
	}

	cfg := genConfig{
		Temperature:     0.4,
		MaxOutputTokens: 1024,
	}
	// thinkingBudget hanya valid untuk model thinking 2.5; mengirimnya ke model lain
	// (mis. 2.0-flash) bisa ditolak 400, jadi hanya disetel saat model 2.5.
	if strings.Contains(g.Model, "2.5") {
		cfg.ThinkingConfig = &thinkingConfig{ThinkingBudget: 0}
	}

	reqBody := generateRequest{
		Contents:         []genContent{{Parts: []genPart{{Text: prompt}}}},
		GenerationConfig: cfg,
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
