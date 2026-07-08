package controller

import (
	"context"
	"net/http"
	"sehatiku-backend/internal/constants"
	"sehatiku-backend/internal/model"

	"github.com/labstack/echo/v5"
)

type nakesUseCase interface {
	ListNakes(ctx context.Context, faskesID string) ([]model.NakesListItem, error)
	GetNakesDetail(ctx context.Context, faskesID, nakesID string) (*model.NakesDetailResponse, error)
	GetMyProfile(ctx context.Context, nakesID string) (*model.NakesDetailResponse, error)
	UpdateNakesStatus(ctx context.Context, faskesID, nakesID string, req *model.UpdateNakesStatusRequest) (*model.UpdateNakesStatusResponse, error)
}

type NakesController struct {
	UseCase nakesUseCase
}

func (c *NakesController) GetMyProfile(ctx *echo.Context) error {
	claims := getNakesClaimsFromCtx(ctx)

	detail, err := c.UseCase.GetMyProfile(ctx.Request().Context(), claims.NakesID)
	if err != nil {
		return mapNakesError(ctx, err)
	}

	return ctx.JSON(http.StatusOK, model.WebResponse[*model.NakesDetailResponse]{
		Message: "detail profil berhasil diambil",
		Data:    detail,
	})
}

func (c *NakesController) ListNakes(ctx *echo.Context) error {
	claims := getFaskesClaimsFromCtx(ctx)

	items, err := c.UseCase.ListNakes(ctx.Request().Context(), claims.FaskesID)
	if err != nil {
		return ctx.JSON(http.StatusInternalServerError, model.WebResponse[any]{
			Message: constants.MsgInternalServerError,
			Errors:  err.Error(),
		})
	}

	return ctx.JSON(http.StatusOK, model.WebResponse[[]model.NakesListItem]{
		Message: "daftar nakes berhasil diambil",
		Data:    items,
	})
}

func (c *NakesController) GetNakesDetail(ctx *echo.Context) error {
	claims := getFaskesClaimsFromCtx(ctx)

	nakesID := ctx.Param("id")
	if nakesID == "" {
		return ctx.JSON(http.StatusBadRequest, model.WebResponse[any]{
			Message: constants.MsgBadRequest,
			Errors:  "nakes id wajib diisi",
		})
	}

	detail, err := c.UseCase.GetNakesDetail(ctx.Request().Context(), claims.FaskesID, nakesID)
	if err != nil {
		return mapNakesError(ctx, err)
	}

	return ctx.JSON(http.StatusOK, model.WebResponse[*model.NakesDetailResponse]{
		Message: "detail nakes berhasil diambil",
		Data:    detail,
	})
}

func (c *NakesController) UpdateStatus(ctx *echo.Context) error {
	claims := getFaskesClaimsFromCtx(ctx)

	nakesID := ctx.Param("id")
	if nakesID == "" {
		return ctx.JSON(http.StatusBadRequest, model.WebResponse[any]{
			Message: constants.MsgBadRequest,
			Errors:  "nakes id wajib diisi",
		})
	}

	req := new(model.UpdateNakesStatusRequest)
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

	resp, err := c.UseCase.UpdateNakesStatus(ctx.Request().Context(), claims.FaskesID, nakesID, req)
	if err != nil {
		return mapNakesError(ctx, err)
	}

	return ctx.JSON(http.StatusOK, model.WebResponse[*model.UpdateNakesStatusResponse]{
		Message: "status nakes berhasil diperbarui",
		Data:    resp,
	})
}
