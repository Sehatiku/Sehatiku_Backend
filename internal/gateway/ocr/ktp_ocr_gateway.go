package ocr

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"time"

	"go.uber.org/zap"
)

var (
	ErrOCRKTPUnreadable = errors.New("KTP tidak terbaca, coba unggah foto yang lebih jelas")
	ErrOCRBadRequest    = errors.New("file tidak valid untuk diproses OCR")
	ErrOCRUnauthorized  = errors.New("OCR API key tidak valid")
	ErrOCRUpstream      = errors.New("layanan OCR tidak tersedia sementara")
)

type KTPOCRGateway struct {
	APIKey     string
	BaseURL    string
	HTTPClient *http.Client
	Log        *zap.Logger
}

func New(apiKey, baseURL string, log *zap.Logger) *KTPOCRGateway {
	return &KTPOCRGateway{
		APIKey:  apiKey,
		BaseURL: baseURL,
		HTTPClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		Log: log,
	}
}

type KTPOCRResult struct {
	NIK              string `json:"nik"`
	Nama             string `json:"nama"`
	TempatLahir      string `json:"tempat_lahir"`
	TanggalLahir     string `json:"tanggal_lahir"`
	JenisKelamin     string `json:"jenis_kelamin"`
	Alamat           string `json:"alamat"`
	RTRW             string `json:"rt_rw"`
	Kelurahan        string `json:"kelurahan"`
	Kecamatan        string `json:"kecamatan"`
	Kota             string `json:"kota"`
	Provinsi         string `json:"provinsi"`
	Agama            string `json:"agama"`
	StatusPerkawinan string `json:"status_perkawinan"`
	Pekerjaan        string `json:"pekerjaan"`
	Kewarganegaraan  string `json:"kewarganegaraan"`
	BerlakuHingga    string `json:"berlaku_hingga"`
}

type ocrAPIResponse struct {
	IsSuccess bool          `json:"is_success"`
	Message   string        `json:"message"`
	Data      *KTPOCRResult `json:"data"`
}

func (g *KTPOCRGateway) ExtractKTP(ctx context.Context, file multipart.File, filename string) (*KTPOCRResult, error) {
	var buf bytes.Buffer
	writer := multipart.NewWriter(&buf)

	part, err := writer.CreateFormFile("file", filepath.Base(filename))
	if err != nil {
		return nil, fmt.Errorf("creating form file: %w", err)
	}
	if _, err = io.Copy(part, file); err != nil {
		return nil, fmt.Errorf("copying file to form: %w", err)
	}
	writer.Close()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, g.BaseURL+"/ocr/ktp-extract", &buf)
	if err != nil {
		return nil, fmt.Errorf("creating OCR request: %w", err)
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("x-api-co-id", g.APIKey)

	resp, err := g.HTTPClient.Do(req)
	if err != nil {
		g.Log.Warn("OCR API unreachable", zap.Error(err))
		return nil, fmt.Errorf("%w: %v", ErrOCRUpstream, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading OCR response: %w", err)
	}

	switch resp.StatusCode {
	case http.StatusOK:
		// handled below
	case http.StatusUnprocessableEntity:
		return nil, ErrOCRKTPUnreadable
	case http.StatusBadRequest:
		return nil, ErrOCRBadRequest
	case http.StatusUnauthorized:
		g.Log.Error("OCR API key rejected by upstream")
		return nil, ErrOCRUnauthorized
	default:
		g.Log.Warn("unexpected OCR API status", zap.Int("status", resp.StatusCode), zap.String("body", string(body)))
		return nil, fmt.Errorf("%w: status %d", ErrOCRUpstream, resp.StatusCode)
	}

	var apiResp ocrAPIResponse
	if err := json.Unmarshal(body, &apiResp); err != nil {
		return nil, fmt.Errorf("parsing OCR response: %w", err)
	}
	if !apiResp.IsSuccess || apiResp.Data == nil {
		return nil, ErrOCRKTPUnreadable
	}

	g.Log.Info("KTP OCR successful", zap.String("nik", apiResp.Data.NIK))
	return apiResp.Data, nil
}
