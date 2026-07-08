package controller

import (
	"context"
	"net/http"
	"sehatiku-backend/internal/constants"
	"sehatiku-backend/internal/model"

	"github.com/labstack/echo/v5"
)

type healthLogUseCase interface {
	CreateHealthLog(ctx context.Context, patientID, idempotencyKey string, req *model.CreateHealthLogRequest) (*model.HealthLogResponse, error)
}

type HealthLogController struct {
	UseCase healthLogUseCase
}

// Create menangani POST /api/v1/patients/health-logs — input satu pengukuran harian pasien.
// patient_id diambil dari JWT (tidak pernah dari body). Header Idempotency-Key wajib untuk
// dedupe double-tap di koneksi flaky (docs/api_guide.md §7).
func (c *HealthLogController) Create(ctx *echo.Context) error {
	claims := getPatientClaimsFromCtx(ctx)

	idempotencyKey := ctx.Request().Header.Get("Idempotency-Key")
	if idempotencyKey == "" {
		return ctx.JSON(http.StatusBadRequest, model.WebResponse[any]{
			Message: constants.MsgBadRequest,
			Errors:  "Idempotency-Key header wajib diisi",
		})
	}

	req := new(model.CreateHealthLogRequest)
	if err := ctx.Bind(req); err != nil {
		return ctx.JSON(http.StatusBadRequest, model.WebResponse[any]{
			Message: constants.MsgBadRequest,
			Errors:  err.Error(),
		})
	}
	if err := ctx.Validate(req); err != nil {
		return ctx.JSON(http.StatusBadRequest, model.WebResponse[any]{
			Message: constants.MsgValidationError,
			Errors:  err.Error(),
		})
	}

	resp, err := c.UseCase.CreateHealthLog(ctx.Request().Context(), claims.PatientID, idempotencyKey, req)
	if err != nil {
		return mapHealthLogError(ctx, err)
	}

	return ctx.JSON(http.StatusCreated, model.WebResponse[*model.HealthLogResponse]{
		Message: "data harian berhasil dicatat",
		Data:    resp,
	})
}
