package controller

import (
	"context"
	"net/http"
	"sehatiku-backend/internal/delivery/http/report"
	"sehatiku-backend/internal/model"
	"strconv"

	"github.com/labstack/echo/v5"
)

type patientSummaryUseCase interface {
	GetPatientSummary(ctx context.Context, patientID string, window int) (*model.SummaryResponse, error)
	GetNakesPatientSummary(ctx context.Context, faskesID, patientID string, window int) (*model.SummaryResponse, error)
	GetNakesPatientBrief(ctx context.Context, faskesID, patientID string) (*model.PreVisitBriefResponse, error)
	GetNakesPatientBriefReport(ctx context.Context, faskesID, patientID string) (*model.BriefReportData, error)
}

type SummaryController struct {
	UseCase patientSummaryUseCase
}

// GetPatientSummary — pasien membaca ringkasan kesehatannya sendiri (?window=7|14|30).
func (c *SummaryController) GetPatientSummary(ctx *echo.Context) error {
	claims := getPatientClaimsFromCtx(ctx)
	window := parseWindowParam(ctx)

	data, err := c.UseCase.GetPatientSummary(ctx.Request().Context(), claims.PatientID, window)
	if err != nil {
		return mapSummaryError(ctx, err)
	}

	return ctx.JSON(http.StatusOK, model.WebResponse[*model.SummaryResponse]{
		Message: "ringkasan kesehatan berhasil diambil",
		Data:    data,
	})
}

// GetNakesPatientSummary — nakes membaca ringkasan klinis satu pasien (tenancy via JWT).
func (c *SummaryController) GetNakesPatientSummary(ctx *echo.Context) error {
	claims := getNakesClaimsFromCtx(ctx)
	patientID := ctx.Param("id")
	if patientID == "" {
		return ctx.JSON(http.StatusBadRequest, model.WebResponse[any]{
			Message: "bad request",
			Errors:  "patient id wajib diisi",
		})
	}
	window := parseWindowParam(ctx)

	data, err := c.UseCase.GetNakesPatientSummary(ctx.Request().Context(), claims.FaskesID, patientID, window)
	if err != nil {
		return mapSummaryError(ctx, err)
	}

	return ctx.JSON(http.StatusOK, model.WebResponse[*model.SummaryResponse]{
		Message: "ringkasan kesehatan pasien berhasil diambil",
		Data:    data,
	})
}

// GetNakesPatientBrief — Pre-Visit Prolanis Brief: dossier 30 hari satu pasien untuk
// dokter menjelang kontrol bulanan (tenancy via JWT, tanpa query param).
func (c *SummaryController) GetNakesPatientBrief(ctx *echo.Context) error {
	claims := getNakesClaimsFromCtx(ctx)
	patientID := ctx.Param("id")
	if patientID == "" {
		return ctx.JSON(http.StatusBadRequest, model.WebResponse[any]{
			Message: "bad request",
			Errors:  "patient id wajib diisi",
		})
	}

	data, err := c.UseCase.GetNakesPatientBrief(ctx.Request().Context(), claims.FaskesID, patientID)
	if err != nil {
		return mapSummaryError(ctx, err)
	}

	return ctx.JSON(http.StatusOK, model.WebResponse[*model.PreVisitBriefResponse]{
		Message: "pre-visit brief pasien berhasil diambil",
		Data:    data,
	})
}

// GetNakesPatientBriefReport — laporan HTML siap-cetak dari Pre-Visit Brief (Print→PDF).
// Sukses = text/html; error tetap JSON (via mapSummaryError) seperti endpoint brief.
func (c *SummaryController) GetNakesPatientBriefReport(ctx *echo.Context) error {
	claims := getNakesClaimsFromCtx(ctx)
	patientID := ctx.Param("id")
	if patientID == "" {
		return ctx.JSON(http.StatusBadRequest, model.WebResponse[any]{
			Message: "bad request",
			Errors:  "patient id wajib diisi",
		})
	}

	data, err := c.UseCase.GetNakesPatientBriefReport(ctx.Request().Context(), claims.FaskesID, patientID)
	if err != nil {
		return mapSummaryError(ctx, err)
	}

	html, err := report.RenderBrief(data)
	if err != nil {
		return mapSummaryError(ctx, err)
	}

	return ctx.HTML(http.StatusOK, html)
}

// parseWindowParam membaca query param window. Kosong -> default 7; nilai non-integer -> 0
// (akan ditolak usecase sebagai ErrInvalidWindow).
func parseWindowParam(ctx *echo.Context) int {
	raw := ctx.QueryParam("window")
	if raw == "" {
		return 7
	}
	w, err := strconv.Atoi(raw)
	if err != nil {
		return 0
	}
	return w
}
