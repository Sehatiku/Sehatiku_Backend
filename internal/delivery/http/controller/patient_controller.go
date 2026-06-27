package controller

import (
	"context"
	"net/http"
	"sehatiku-backend/internal/model"
	"strconv"

	"github.com/labstack/echo/v5"
)

type patientListUseCase interface {
	ListPatients(ctx context.Context, faskesID string, page, size int) ([]model.PatientListItem, model.PageMetadata, error)
}

type PatientController struct {
	UseCase patientListUseCase
}

func (c *PatientController) ListPatients(ctx *echo.Context) error {
	claims := getFaskesClaimsFromCtx(ctx)

	page, _ := strconv.Atoi(ctx.QueryParam("page"))
	if page < 1 {
		page = 1
	}
	size, _ := strconv.Atoi(ctx.QueryParam("size"))
	if size < 1 || size > 100 {
		size = 20
	}

	items, paging, err := c.UseCase.ListPatients(ctx.Request().Context(), claims.FaskesID, page, size)
	if err != nil {
		return ctx.JSON(http.StatusInternalServerError, model.WebResponse[any]{
			Message: "internal server error",
			Errors:  err.Error(),
		})
	}

	return ctx.JSON(http.StatusOK, model.WebResponse[[]model.PatientListItem]{
		Message: "daftar pasien berhasil diambil",
		Data:    items,
		Paging:  &paging,
	})
}
