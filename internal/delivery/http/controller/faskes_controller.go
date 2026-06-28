package controller

import (
	"context"
	"net/http"
	"sehatiku-backend/internal/model"

	"github.com/labstack/echo/v5"
)

type faskesUseCase interface {
	GetFaskesProfile(ctx context.Context, faskesID string) (*model.FaskesProfileResponse, error)
}

type FaskesController struct {
	UseCase faskesUseCase
}

func (c *FaskesController) GetProfile(ctx *echo.Context) error {
	claims := getFaskesClaimsFromCtx(ctx)

	profile, err := c.UseCase.GetFaskesProfile(ctx.Request().Context(), claims.FaskesID)
	if err != nil {
		return mapFaskesError(ctx, err)
	}

	return ctx.JSON(http.StatusOK, model.WebResponse[*model.FaskesProfileResponse]{
		Message: "detail faskes berhasil diambil",
		Data:    profile,
	})
}
