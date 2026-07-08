package controller

import (
	"errors"
	"net/http"
	"sehatiku-backend/internal/constants"
	"sehatiku-backend/internal/model"
	"sehatiku-backend/internal/repository"
	"sehatiku-backend/internal/usecase"

	"github.com/labstack/echo/v5"
)

type TokenController struct {
	UseCase *usecase.TokenUseCase
}

func (c *TokenController) Refresh(ctx *echo.Context) error {
	req := new(model.RefreshTokenRequest)
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

	resp, err := c.UseCase.Refresh(ctx.Request().Context(), req)
	if err != nil {
		if errors.Is(err, repository.ErrRefreshTokenInvalid) || errors.Is(err, repository.ErrRefreshTokenReused) {
			return ctx.JSON(http.StatusUnauthorized, model.WebResponse[any]{
				Message: constants.MsgUnauthorized,
				Errors:  "refresh token tidak valid",
			})
		}
		return ctx.JSON(http.StatusInternalServerError, model.WebResponse[any]{
			Message: constants.MsgInternalServerError,
			Errors:  err.Error(),
		})
	}

	return ctx.JSON(http.StatusOK, model.WebResponse[*model.TokenResponse]{
		Message: "token diperbarui",
		Data:    resp,
	})
}

// Logout mengambil role dan user ID dari context yang sudah di-set middleware JWT.
func (c *TokenController) Logout(ctx *echo.Context) error {
	req := new(model.LogoutRequest)
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

	role, _ := ctx.Get("auth_role").(string)
	userID, _ := ctx.Get("auth_user_id").(string)

	if err := c.UseCase.Logout(ctx.Request().Context(), req, role, userID); err != nil {
		return ctx.JSON(http.StatusInternalServerError, model.WebResponse[any]{
			Message: constants.MsgInternalServerError,
			Errors:  err.Error(),
		})
	}

	return ctx.JSON(http.StatusOK, model.WebResponse[any]{
		Message: "logout berhasil",
	})
}
