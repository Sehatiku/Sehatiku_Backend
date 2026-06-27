package controller

import (
	"net/http"
	"sehatiku-backend/internal/model"
	"sehatiku-backend/internal/usecase"

	"github.com/labstack/echo/v5"
)

type NakesAuthController struct {
	UseCase *usecase.NakesAuthUseCase
}

func (c *NakesAuthController) Login(ctx *echo.Context) error {
	req := new(model.NakesLoginRequest)
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

	resp, err := c.UseCase.Login(ctx.Request().Context(), req)
	if err != nil {
		return mapAuthError(ctx, err)
	}

	return ctx.JSON(http.StatusOK, model.WebResponse[*model.NakesLoginResponse]{
		Message: "login berhasil",
		Data:    resp,
	})
}
