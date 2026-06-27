package controller

import (
	"net/http"
	"sehatiku-backend/internal/model"
	"sehatiku-backend/internal/usecase"

	"github.com/labstack/echo/v5"
)

type NakesRegistrationController struct {
	UseCase *usecase.NakesRegistrationUseCase
}

func (c *NakesRegistrationController) ScanKTP(ctx *echo.Context) error {
	fh, err := ctx.FormFile("file")
	if err != nil {
		return ctx.JSON(http.StatusBadRequest, model.WebResponse[any]{
			Message: "bad request",
			Errors:  "field 'file' wajib diisi dengan gambar KTP",
		})
	}

	file, err := fh.Open()
	if err != nil {
		return ctx.JSON(http.StatusInternalServerError, model.WebResponse[any]{
			Message: "internal server error",
			Errors:  err.Error(),
		})
	}
	defer file.Close()

	resp, err := c.UseCase.ScanKTP(ctx.Request().Context(), file, fh.Filename)
	if err != nil {
		return mapRegistrationError(ctx, err)
	}

	return ctx.JSON(http.StatusOK, model.WebResponse[*model.KTPOCRResponse]{
		Message: "KTP berhasil di-scan",
		Data:    resp,
	})
}

func (c *NakesRegistrationController) RegisterNakes(ctx *echo.Context) error {
	claims := getFaskesClaimsFromCtx(ctx)

	req := new(model.NakesRegisterRequest)
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

	resp, err := c.UseCase.RegisterNakes(ctx.Request().Context(), claims.FaskesID, req)
	if err != nil {
		return mapRegistrationError(ctx, err)
	}

	return ctx.JSON(http.StatusOK, model.WebResponse[*model.NakesRegisterResponse]{
		Message: "nakes berhasil didaftarkan",
		Data:    resp,
	})
}

func getFaskesClaimsFromCtx(ctx *echo.Context) *model.FaskesAuthClaims {
	return ctx.Get("faskes_auth").(*model.FaskesAuthClaims)
}
