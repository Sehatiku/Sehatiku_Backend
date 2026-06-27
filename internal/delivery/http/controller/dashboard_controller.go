package controller

import (
	"context"
	"net/http"
	"sehatiku-backend/internal/model"
	"strconv"

	"github.com/labstack/echo/v5"
)

type dashboardSummaryUseCase interface {
	GetSummary(ctx context.Context, faskesID string) (*model.DashboardSummaryResponse, error)
}

type dashboardQueueUseCase interface {
	GetPatientQueue(ctx context.Context, faskesID string, page, size int) ([]model.PatientQueueItem, model.PageMetadata, error)
}

type DashboardController struct {
	SummaryUseCase dashboardSummaryUseCase
	QueueUseCase   dashboardQueueUseCase
}

func (c *DashboardController) GetSummary(ctx *echo.Context) error {
	claims := getNakesClaimsFromCtx(ctx)

	summary, err := c.SummaryUseCase.GetSummary(ctx.Request().Context(), claims.FaskesID)
	if err != nil {
		return ctx.JSON(http.StatusInternalServerError, model.WebResponse[any]{
			Message: "internal server error",
			Errors:  err.Error(),
		})
	}

	return ctx.JSON(http.StatusOK, model.WebResponse[*model.DashboardSummaryResponse]{
		Message: "ringkasan dashboard berhasil diambil",
		Data:    summary,
	})
}

func (c *DashboardController) GetPatientQueue(ctx *echo.Context) error {
	claims := getNakesClaimsFromCtx(ctx)

	page, _ := strconv.Atoi(ctx.QueryParam("page"))
	if page < 1 {
		page = 1
	}
	size, _ := strconv.Atoi(ctx.QueryParam("size"))
	if size < 1 || size > 100 {
		size = 20
	}

	items, paging, err := c.QueueUseCase.GetPatientQueue(ctx.Request().Context(), claims.FaskesID, page, size)
	if err != nil {
		return ctx.JSON(http.StatusInternalServerError, model.WebResponse[any]{
			Message: "internal server error",
			Errors:  err.Error(),
		})
	}

	return ctx.JSON(http.StatusOK, model.WebResponse[[]model.PatientQueueItem]{
		Message: "antrian prioritas pasien berhasil diambil",
		Data:    items,
		Paging:  &paging,
	})
}
