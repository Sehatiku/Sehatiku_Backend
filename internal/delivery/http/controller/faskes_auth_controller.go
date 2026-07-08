package controller

import (
	"errors"
	"net/http"
	"sehatiku-backend/internal/constants"
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

	if err := c.UseCase.Register(ctx.Request().Context(), req); err != nil {
		if errors.Is(err, usecase.ErrUsernameAlreadyExists) || errors.Is(err, usecase.ErrPhoneAlreadyExists) {
			return ctx.JSON(http.StatusConflict, model.WebResponse[any]{
				Message: constants.MsgConflict,
				Errors:  err.Error(),
			})
		}
		return ctx.JSON(http.StatusInternalServerError, model.WebResponse[any]{
			Message: constants.MsgInternalServerError,
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

	return ctx.JSON(http.StatusOK, model.WebResponse[*model.FaskesLoginResponse]{
		Message: "login berhasil",
		Data:    resp,
	})
}
