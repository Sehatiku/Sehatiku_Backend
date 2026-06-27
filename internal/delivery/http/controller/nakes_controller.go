package controller

import (
	"context"
	"net/http"
	"sehatiku-backend/internal/model"

	"github.com/labstack/echo/v5"
)

type nakesListUseCase interface {
	ListNakes(ctx context.Context, faskesID string) ([]model.NakesListItem, error)
}

type NakesController struct {
	UseCase nakesListUseCase
}

func (c *NakesController) ListNakes(ctx *echo.Context) error {
	claims := getFaskesClaimsFromCtx(ctx)

	items, err := c.UseCase.ListNakes(ctx.Request().Context(), claims.FaskesID)
	if err != nil {
		return ctx.JSON(http.StatusInternalServerError, model.WebResponse[any]{
			Message: "internal server error",
			Errors:  err.Error(),
		})
	}

	return ctx.JSON(http.StatusOK, model.WebResponse[[]model.NakesListItem]{
		Message: "daftar nakes berhasil diambil",
		Data:    items,
	})
}
