package controller

import (
	"context"
	"errors"
	"net/http"
	"sehatiku-backend/internal/model"
	"sehatiku-backend/internal/usecase"
	"strconv"

	"github.com/labstack/echo/v5"
)

type recordUseCase interface {
	CreateRecord(ctx context.Context, patientID string, req *model.CreateRecordRequest) (*model.CreateRecordResponse, error)
	GetHistory(ctx context.Context, patientID string, limit int) ([]model.RecordHistoryItem, error)
	GetTodayStatus(ctx context.Context, patientID string) (*model.TodayStatusResponse, error)
}

type RecordController struct {
	UseCase recordUseCase
}

func (c *RecordController) Create(ctx *echo.Context) error {
	claims := getPatientClaimsFromCtx(ctx)

	req := new(model.CreateRecordRequest)
	if err := ctx.Bind(req); err != nil {
		return ctx.JSON(http.StatusBadRequest, model.WebResponse[any]{
			Message: "bad request",
			Errors:  err.Error(),
		})
	}
	if err := ctx.Validate(req); err != nil {
		return ctx.JSON(http.StatusBadRequest, model.WebResponse[any]{
			Message: "validation error",
			Errors:  err.Error(),
		})
	}

	data, err := c.UseCase.CreateRecord(ctx.Request().Context(), claims.PatientID, req)
	if err != nil {
		if errors.Is(err, usecase.ErrInvalidHealthLog) || errors.Is(err, usecase.ErrNoMetricProvided) {
			return ctx.JSON(http.StatusBadRequest, model.WebResponse[any]{
				Message: "bad request",
				Errors:  err.Error(),
			})
		}
		return ctx.JSON(http.StatusInternalServerError, model.WebResponse[any]{
			Message: "internal server error",
			Errors:  err.Error(),
		})
	}

	return ctx.JSON(http.StatusCreated, model.WebResponse[*model.CreateRecordResponse]{
		Message: "catatan harian berhasil disimpan",
		Data:    data,
	})
}

// GetTodayStatus menangani GET /api/v1/patients/records/today-status.
// Mengembalikan apakah pasien sudah mengisi data harian hari ini (WIB) agar mobile
// dapat memunculkan pop-up pengingat.
func (c *RecordController) GetTodayStatus(ctx *echo.Context) error {
	claims := getPatientClaimsFromCtx(ctx)

	data, err := c.UseCase.GetTodayStatus(ctx.Request().Context(), claims.PatientID)
	if err != nil {
		return ctx.JSON(http.StatusInternalServerError, model.WebResponse[any]{
			Message: "internal server error",
			Errors:  err.Error(),
		})
	}

	return ctx.JSON(http.StatusOK, model.WebResponse[*model.TodayStatusResponse]{
		Message: "status input harian berhasil diambil",
		Data:    data,
	})
}

func (c *RecordController) GetHistory(ctx *echo.Context) error {
	claims := getPatientClaimsFromCtx(ctx)

	limit := 7
	if limitStr := ctx.QueryParam("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 {
			limit = l
		}
	}

	data, err := c.UseCase.GetHistory(ctx.Request().Context(), claims.PatientID, limit)
	if err != nil {
		return ctx.JSON(http.StatusInternalServerError, model.WebResponse[any]{
			Message: "internal server error",
			Errors:  err.Error(),
		})
	}

	return ctx.JSON(http.StatusOK, model.WebResponse[[]model.RecordHistoryItem]{
		Message: "riwayat catatan berhasil diambil",
		Data:    data,
	})
}
