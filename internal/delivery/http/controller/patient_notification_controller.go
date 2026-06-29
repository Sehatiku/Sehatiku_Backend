package controller

import (
	"context"
	"net/http"
	"sehatiku-backend/internal/model"

	"github.com/labstack/echo/v5"
)

type patientNotificationUseCase interface {
	GetNotifications(ctx context.Context, patientID string) ([]model.PatientNotificationResponse, error)
}

type PatientNotificationController struct {
	UseCase patientNotificationUseCase
}

func (c *PatientNotificationController) GetNotifications(ctx *echo.Context) error {
	claims := getPatientClaimsFromCtx(ctx)

	data, err := c.UseCase.GetNotifications(ctx.Request().Context(), claims.PatientID)
	if err != nil {
		return ctx.JSON(http.StatusInternalServerError, model.WebResponse[any]{Message: "internal server error", Errors: err.Error()})
	}

	return ctx.JSON(http.StatusOK, model.WebResponse[[]model.PatientNotificationResponse]{
		Message: "notifikasi berhasil diambil",
		Data:    data,
	})
}
