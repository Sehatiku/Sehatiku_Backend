package controller

import (
	"context"
	"net/http"
	"sehatiku-backend/internal/constants"
	"sehatiku-backend/internal/model"

	"github.com/labstack/echo/v5"
)

type devicePushTokenUseCase interface {
	Register(ctx context.Context, patientID, token, platform string) error
	Deregister(ctx context.Context, patientID, token string) error
}

type DevicePushTokenController struct {
	UseCase devicePushTokenUseCase
}

func (c *DevicePushTokenController) Register(ctx *echo.Context) error {
	claims := getPatientClaimsFromCtx(ctx)

	req := new(model.RegisterDeviceTokenRequest)
	if err := ctx.Bind(req); err != nil {
		return ctx.JSON(http.StatusBadRequest, model.WebResponse[any]{Message: constants.MsgBadRequest, Errors: err.Error()})
	}
	if err := ctx.Validate(req); err != nil {
		return ctx.JSON(http.StatusBadRequest, model.WebResponse[any]{Message: constants.MsgValidationError, Errors: err.Error()})
	}

	if err := c.UseCase.Register(ctx.Request().Context(), claims.PatientID, req.Token, req.Platform); err != nil {
		return ctx.JSON(http.StatusInternalServerError, model.WebResponse[any]{Message: constants.MsgInternalServerError, Errors: err.Error()})
	}
	return ctx.JSON(http.StatusOK, model.WebResponse[any]{Message: "device token berhasil didaftarkan", Data: nil})
}

func (c *DevicePushTokenController) Deregister(ctx *echo.Context) error {
	claims := getPatientClaimsFromCtx(ctx)

	req := new(model.DeregisterDeviceTokenRequest)
	if err := ctx.Bind(req); err != nil {
		return ctx.JSON(http.StatusBadRequest, model.WebResponse[any]{Message: constants.MsgBadRequest, Errors: err.Error()})
	}
	if err := ctx.Validate(req); err != nil {
		return ctx.JSON(http.StatusBadRequest, model.WebResponse[any]{Message: constants.MsgValidationError, Errors: err.Error()})
	}

	if err := c.UseCase.Deregister(ctx.Request().Context(), claims.PatientID, req.Token); err != nil {
		return ctx.JSON(http.StatusInternalServerError, model.WebResponse[any]{Message: constants.MsgInternalServerError, Errors: err.Error()})
	}
	return ctx.JSON(http.StatusOK, model.WebResponse[any]{Message: "device token berhasil dihapus", Data: nil})
}
