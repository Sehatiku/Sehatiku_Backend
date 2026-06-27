package controller

import (
	"errors"
	"net/http"
	"sehatiku-backend/internal/model"
	"sehatiku-backend/internal/usecase"

	"github.com/labstack/echo/v5"
)

type FaskesAuthController struct {
	UseCase *usecase.FaskesAuthUseCase
}

func (c *FaskesAuthController) Register(ctx *echo.Context) error {
	req := new(model.FaskesRegisterRequest)
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

	if err := c.UseCase.Register(ctx.Request().Context(), req); err != nil {
		if errors.Is(err, usecase.ErrUsernameAlreadyExists) {
			return ctx.JSON(http.StatusConflict, model.WebResponse[any]{
				Message: "conflict",
				Errors:  err.Error(),
			})
		}
		return ctx.JSON(http.StatusInternalServerError, model.WebResponse[any]{
			Message: "internal server error",
			Errors:  err.Error(),
		})
	}

	return ctx.JSON(http.StatusOK, model.WebResponse[any]{
		Message: "faskes berhasil didaftarkan",
	})
}

func (c *FaskesAuthController) Login(ctx *echo.Context) error {
	req := new(model.FaskesLoginRequest)
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

	return ctx.JSON(http.StatusOK, model.WebResponse[*model.FaskesLoginResponse]{
		Message: "login berhasil",
		Data:    resp,
	})
}
