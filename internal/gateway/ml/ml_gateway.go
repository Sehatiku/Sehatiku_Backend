// Package ml is the HTTP gateway to the Sehatiku ML inference service
// (FastAPI: NER + TKPI + XGBoost/SHAP), deployed separately on Cloud Run / Railway.
// It mirrors the structure of internal/gateway/ocr (KTPOCRGateway): a small typed
// client with a constructor, typed errors, and a status-code switch.
//
// Wiring (add to internal/config/app.go BootStrap, next to the OCR gateway — only
// after a usecase actually consumes it, otherwise Go reports an unused variable):
//
//	mlgw "sehatiku-backend/internal/gateway/ml"
//	...
//	mlBaseURL := config.Config.GetString("ML_API_BASE_URL") // e.g. https://sehatiku-ml-xxxx.a.run.app
//	mlGateway := mlgw.New(mlBaseURL, config.Config.GetString("ML_API_KEY"), config.Log)
//
// Env vars (add to .env / .env.example):
//
//	ML_API_BASE_URL=https://<cloud-run-url>
//	ML_API_KEY=<same secret configured on the ML service>
package ml

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"go.uber.org/zap"
)

var (
	ErrMLBadRequest   = errors.New("permintaan tidak valid untuk layanan ML")
	ErrMLUnauthorized = errors.New("ML API key tidak valid")
	ErrMLUpstream     = errors.New("layanan ML tidak tersedia sementara")
)

// MLGateway is an HTTP client for the ML inference service.
type MLGateway struct {
	BaseURL    string
	APIKey     string
	HTTPClient *http.Client
	Log        *zap.Logger
}

// New builds a gateway. baseURL is the ML service root (no trailing slash needed);
// apiKey is sent as the X-API-Key header (leave empty to call an unsecured service).
func New(baseURL, apiKey string, log *zap.Logger) *MLGateway {
	return &MLGateway{
		BaseURL: strings.TrimRight(baseURL, "/"),
		APIKey:  apiKey,
		HTTPClient: &http.Client{
			// Generous: a scaled-to-zero Cloud Run instance can cold-start (container
			// boot + ~30s model load) on the first request after idle.
			Timeout: 90 * time.Second,
		},
		Log: log,
	}
}

// --- /extract response shapes (see docs/integration in the ML repo) ---

type ExtractFood struct {
	Query      string  `json:"query"`
	Matched    string  `json:"matched"`
	MatchScore int     `json:"match_score"`
	Portion    float64 `json:"portion"`
	Kcal       float64 `json:"kcal"`
	CarbsG     float64 `json:"carbs_g"`
	SodiumMg   float64 `json:"sodium_mg"`
}

type BloodPressure struct {
	Systolic  float64 `json:"systolic"`
	Diastolic float64 `json:"diastolic"`
}

type Totals struct {
	Kcal     float64 `json:"kcal"`
	CarbsG   float64 `json:"carbs_g"`
	SodiumMg float64 `json:"sodium_mg"`
}

// HealthLog is one ready-to-insert health_logs row. For metric_type "food"/"bp" the
// payload is in ValueJSONB; for "glucose" it is in ValueNumeric.
type HealthLog struct {
	MetricType   string          `json:"metric_type"`
	ValueNumeric *float64        `json:"value_numeric,omitempty"`
	ValueJSONB   json.RawMessage `json:"value_jsonb,omitempty"`
}

type ExtractResult struct {
	Text           string         `json:"text"`
	Foods          []ExtractFood  `json:"foods"`
	UnmatchedFoods []string       `json:"unmatched_foods"`
	Totals         Totals         `json:"totals"`
	BloodPressure  *BloodPressure `json:"blood_pressure"`
	Glucose        *float64       `json:"glucose"`
	HealthLogs     []HealthLog    `json:"health_logs"`
}

// --- /predict_health_score shapes ---

type Baseline struct {
	AgeYears       int     `json:"age_years"`
	Sex            string  `json:"sex"` // "male"/"female" accepted by the ML service
	BMI            float64 `json:"bmi"`
	EGFR           float64 `json:"eGFR"`
	HbA1cPct       float64 `json:"hba1c_pct"`
	SystolicBPmmHg float64 `json:"systolic_bp_mmhg"`
}

type DailyAverage struct {
	GlucoseMeanRoll7 float64 `json:"glucose_mean_roll7"`
	GlucoseCVRoll7   float64 `json:"glucose_cv_roll7"`
	SystolicRoll7    float64 `json:"systolic_roll7"`
	SodiumRoll7      float64 `json:"sodium_roll7"`
	SleepRoll7       float64 `json:"sleep_roll7"`
	ActivityPctRoll7 float64 `json:"activity_pct_roll7"`
	StressRoll7      float64 `json:"stress_roll7"`
	CarbsRoll7       float64 `json:"carbs_roll7"`
}

type PredictRequest struct {
	Baseline       Baseline     `json:"baseline"`
	Daily7DAverage DailyAverage `json:"daily_7d_average"`
}

type PredictResult struct {
	HealthScore  float64  `json:"health_score"`
	Status       string   `json:"status"`       // DB enum: aman / waswas / bahaya
	StatusLabel  string   `json:"status_label"` // display: Sehat / Waswas / Parah
	Message      string   `json:"message"`
	TopPenalties []string `json:"top_penalties"`
}

// ExtractChat turns a patient chat message into entities + TKPI nutrition.
func (g *MLGateway) ExtractChat(ctx context.Context, text string) (*ExtractResult, error) {
	var out ExtractResult
	if err := g.postJSON(ctx, "/extract", map[string]string{"text": text}, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// PredictHealthScore scores a patient's baseline + 7-day rolled features.
func (g *MLGateway) PredictHealthScore(ctx context.Context, req PredictRequest) (*PredictResult, error) {
	var out PredictResult
	if err := g.postJSON(ctx, "/predict_health_score", req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// postJSON POSTs reqBody as JSON to BaseURL+path and decodes the response into out.
func (g *MLGateway) postJSON(ctx context.Context, path string, reqBody, out any) error {
	payload, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("encoding ML request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, g.BaseURL+path, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("creating ML request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if g.APIKey != "" {
		req.Header.Set("X-API-Key", g.APIKey)
	}

	resp, err := g.HTTPClient.Do(req)
	if err != nil {
		g.Log.Warn("ML API unreachable", zap.String("path", path), zap.Error(err))
		return fmt.Errorf("%w: %v", ErrMLUpstream, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("reading ML response: %w", err)
	}

	switch resp.StatusCode {
	case http.StatusOK:
		// handled below
	case http.StatusUnauthorized:
		g.Log.Error("ML API key rejected", zap.String("path", path))
		return ErrMLUnauthorized
	case http.StatusBadRequest, http.StatusUnprocessableEntity:
		g.Log.Warn("ML upstream: bad request", zap.String("path", path), zap.String("body", string(body)))
		return fmt.Errorf("%w: %s", ErrMLBadRequest, string(body))
	default:
		g.Log.Warn("unexpected ML API status", zap.Int("status", resp.StatusCode), zap.String("body", string(body)))
		return fmt.Errorf("%w: status %d", ErrMLUpstream, resp.StatusCode)
	}

	if err := json.Unmarshal(body, out); err != nil {
		return fmt.Errorf("parsing ML response: %w", err)
	}
	return nil
}
