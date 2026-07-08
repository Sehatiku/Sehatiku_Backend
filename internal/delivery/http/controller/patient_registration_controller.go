package controller

import (
	"net/http"
	"sehatiku-backend/internal/constants"
	"sehatiku-backend/internal/model"
	"sehatiku-backend/internal/usecase"

	"github.com/labstack/echo/v5"
)

type PatientRegistrationController struct {
	UseCase *usecase.PatientRegistrationUseCase
}

func (c *PatientRegistrationController) ScanKTP(ctx *echo.Context) error {
	fh, err := ctx.FormFile("file")
	if err != nil {
		return ctx.JSON(http.StatusBadRequest, model.WebResponse[any]{
			Message: constants.MsgBadRequest,
			Errors:  "field 'file' wajib diisi dengan gambar KTP",
		})
	}

	file, err := fh.Open()
	if err != nil {
		return ctx.JSON(http.StatusInternalServerError, model.WebResponse[any]{
			Message: constants.MsgInternalServerError,
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

func (c *PatientRegistrationController) RegisterPatient(ctx *echo.Context) error {
	claims := getFaskesClaimsFromCtx(ctx)

	req := new(model.PatientRegisterRequest)
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

	resp, err := c.UseCase.RegisterPatient(
		ctx.Request().Context(),
		claims.FaskesID,
		req,
	)
	if err != nil {
		return mapRegistrationError(ctx, err)
	}

	return ctx.JSON(http.StatusOK, model.WebResponse[*model.PatientRegisterResponse]{
		Message: "pasien berhasil didaftarkan",
		Data:    resp,
	})
}

func getNakesClaimsFromCtx(ctx *echo.Context) *model.NakesAuthClaims {
	return ctx.Get("nakes_auth").(*model.NakesAuthClaims)
}
