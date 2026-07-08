package controller

import (
	"net/http"
	"sehatiku-backend/internal/constants"
	"sehatiku-backend/internal/model"
	"sehatiku-backend/internal/usecase"

	"github.com/labstack/echo/v5"
)

type PatientAuthController struct {
	UseCase *usecase.PatientAuthUseCase
}

func (c *PatientAuthController) Login(ctx *echo.Context) error {
	req := new(model.PatientLoginRequest)
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

	resp, err := c.UseCase.Login(ctx.Request().Context(), req)
	if err != nil {
		return mapAuthError(ctx, err)
	}

	return ctx.JSON(http.StatusOK, model.WebResponse[*model.PatientLoginResponse]{
		Message: "login berhasil",
		Data:    resp,
	})
}
