package controller

import (
	"context"
	"net/http"
	"sehatiku-backend/internal/model"
	"strconv"

	"github.com/labstack/echo/v5"
)

type baselineUseCase interface {
	GetLatestBaseline(ctx context.Context, faskesID, patientID string) (*model.BaselineDetailResponse, error)
	CreateBaseline(ctx context.Context, faskesID, patientID string, req *model.CreateBaselineRequest) (*model.BaselineDetailResponse, error)
	ListBaselineHistoryForFaskes(ctx context.Context, faskesID, patientID string, page, size int) ([]model.BaselineHistoryItem, model.PageMetadata, error)
	ListBaselineHistoryForPatient(ctx context.Context, patientID string, page, size int) ([]model.BaselineHistoryItem, model.PageMetadata, error)
}

type BaselineController struct {
	UseCase baselineUseCase
}

// GetLatest (faskes) — baseline terlengkap terbaru pasien, untuk pre-fill form update.
func (c *BaselineController) GetLatest(ctx *echo.Context) error {
	claims := getFaskesClaimsFromCtx(ctx)

	patientID := ctx.Param("id")
	if patientID == "" {
		return ctx.JSON(http.StatusBadRequest, model.WebResponse[any]{
			Message: "bad request",
			Errors:  "patient id wajib diisi",
		})
	}

	detail, err := c.UseCase.GetLatestBaseline(ctx.Request().Context(), claims.FaskesID, patientID)
	if err != nil {
		return mapBaselineError(ctx, err)
	}

	return ctx.JSON(http.StatusOK, model.WebResponse[*model.BaselineDetailResponse]{
		Message: "baseline terbaru berhasil diambil",
		Data:    detail,
	})
}

// Create (faskes) — catat versi baseline baru (insert-only). 201 Created.
func (c *BaselineController) Create(ctx *echo.Context) error {
	claims := getFaskesClaimsFromCtx(ctx)

	patientID := ctx.Param("id")
	if patientID == "" {
		return ctx.JSON(http.StatusBadRequest, model.WebResponse[any]{
			Message: "bad request",
			Errors:  "patient id wajib diisi",
		})
	}

	req := new(model.CreateBaselineRequest)
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

	detail, err := c.UseCase.CreateBaseline(ctx.Request().Context(), claims.FaskesID, patientID, req)
	if err != nil {
		return mapBaselineError(ctx, err)
	}

	return ctx.JSON(http.StatusCreated, model.WebResponse[*model.BaselineDetailResponse]{
		Message: "baseline berhasil dicatat",
		Data:    detail,
	})
}

// GetHistory (faskes) — progress baseline pasien (metrik kunci), paginated.
func (c *BaselineController) GetHistory(ctx *echo.Context) error {
	claims := getFaskesClaimsFromCtx(ctx)

	patientID := ctx.Param("id")
	if patientID == "" {
		return ctx.JSON(http.StatusBadRequest, model.WebResponse[any]{
			Message: "bad request",
			Errors:  "patient id wajib diisi",
		})
	}

	page, size := parsePageSize(ctx)
	items, paging, err := c.UseCase.ListBaselineHistoryForFaskes(ctx.Request().Context(), claims.FaskesID, patientID, page, size)
	if err != nil {
		return mapBaselineError(ctx, err)
	}

	return ctx.JSON(http.StatusOK, model.WebResponse[[]model.BaselineHistoryItem]{
		Message: "riwayat baseline berhasil diambil",
		Data:    items,
		Paging:  &paging,
	})
}

// GetMyHistory (patient) — pasien melihat progress baseline sendiri, paginated.
func (c *BaselineController) GetMyHistory(ctx *echo.Context) error {
	claims := getPatientClaimsFromCtx(ctx)

	page, size := parsePageSize(ctx)
	items, paging, err := c.UseCase.ListBaselineHistoryForPatient(ctx.Request().Context(), claims.PatientID, page, size)
	if err != nil {
		return mapBaselineError(ctx, err)
	}

	return ctx.JSON(http.StatusOK, model.WebResponse[[]model.BaselineHistoryItem]{
		Message: "riwayat baseline berhasil diambil",
		Data:    items,
		Paging:  &paging,
	})
}

// parsePageSize membaca query param page/size dengan default page=1, size=20 (maks 100).
func parsePageSize(ctx *echo.Context) (int, int) {
	page, _ := strconv.Atoi(ctx.QueryParam("page"))
	if page < 1 {
		page = 1
	}
	size, _ := strconv.Atoi(ctx.QueryParam("size"))
	if size < 1 || size > 100 {
		size = 20
	}
	return page, size
}
