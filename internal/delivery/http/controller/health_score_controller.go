package controller

import (
	"context"
	"net/http"
	"time"

	"sehatiku-backend/internal/gateway/ml"
	"sehatiku-backend/internal/model"

	"github.com/labstack/echo/v5"
)

type scoringUseCase interface {
	ScorePatient(ctx context.Context, patientID string) (*ml.PredictResult, error)
}

type HealthScoreController struct {
	UseCase scoringUseCase
}

// Get menangani GET /api/v1/patients/health-score — hitung roll-7 dari health_logs,
// panggil ML /predict_health_score, simpan risk_scores, lalu kembalikan skor + faktor
// SHAP. patient_id diambil dari JWT (tidak pernah dari query/body).
func (c *HealthScoreController) Get(ctx *echo.Context) error {
	claims := getPatientClaimsFromCtx(ctx)

	res, err := c.UseCase.ScorePatient(ctx.Request().Context(), claims.PatientID)
	if err != nil {
		return mapHealthScoreError(ctx, err)
	}

	return ctx.JSON(http.StatusOK, model.WebResponse[*model.HealthScoreResponse]{
		Message: "health score berhasil dihitung",
		Data: &model.HealthScoreResponse{
			HealthScore:  res.HealthScore,
			Status:       res.Status,
			StatusLabel:  res.StatusLabel,
			Message:      res.Message,
			TopPenalties: res.TopPenalties,
			ScoredAt:     time.Now(),
		},
	})
}
