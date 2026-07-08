package controller

import (
	"context"
	"net/http"
	"sehatiku-backend/internal/constants"
	"sehatiku-backend/internal/model"
	"strings"

	"github.com/labstack/echo/v5"
)

type patientNotificationUseCase interface {
	GetNotifications(ctx context.Context, patientID string) ([]model.PatientNotificationResponse, error)
	GetUnreadCount(ctx context.Context, patientID string) (*model.UnreadCountResponse, error)
	MarkRead(ctx context.Context, id, patientID string) error
	MarkAllRead(ctx context.Context, patientID string) (*model.MarkAllReadResponse, error)
}

type PatientNotificationController struct {
	UseCase patientNotificationUseCase
}

func (c *PatientNotificationController) GetNotifications(ctx *echo.Context) error {
	claims := getPatientClaimsFromCtx(ctx)

	data, err := c.UseCase.GetNotifications(ctx.Request().Context(), claims.PatientID)
	if err != nil {
		return ctx.JSON(http.StatusInternalServerError, model.WebResponse[any]{Message: constants.MsgInternalServerError, Errors: err.Error()})
	}

	return ctx.JSON(http.StatusOK, model.WebResponse[[]model.PatientNotificationResponse]{
		Message: "notifikasi berhasil diambil",
		Data:    data,
	})
}

func (c *PatientNotificationController) GetUnreadCount(ctx *echo.Context) error {
	claims := getPatientClaimsFromCtx(ctx)

	data, err := c.UseCase.GetUnreadCount(ctx.Request().Context(), claims.PatientID)
	if err != nil {
		return ctx.JSON(http.StatusInternalServerError, model.WebResponse[any]{Message: constants.MsgInternalServerError, Errors: err.Error()})
	}

	return ctx.JSON(http.StatusOK, model.WebResponse[*model.UnreadCountResponse]{
		Message: "jumlah notifikasi belum dibaca",
		Data:    data,
	})
}

func (c *PatientNotificationController) MarkRead(ctx *echo.Context) error {
	claims := getPatientClaimsFromCtx(ctx)
	id := ctx.Param("id")
	if id == "" {
		return ctx.JSON(http.StatusBadRequest, model.WebResponse[any]{Message: "notification id wajib diisi"})
	}

	if err := c.UseCase.MarkRead(ctx.Request().Context(), id, claims.PatientID); err != nil {
		if strings.Contains(err.Error(), "not found") {
			return ctx.JSON(http.StatusNotFound, model.WebResponse[any]{Message: "notifikasi tidak ditemukan", Errors: err.Error()})
		}
		return ctx.JSON(http.StatusInternalServerError, model.WebResponse[any]{Message: constants.MsgInternalServerError, Errors: err.Error()})
	}

	return ctx.JSON(http.StatusOK, model.WebResponse[any]{
		Message: "notifikasi ditandai sudah dibaca",
		Data:    nil,
	})
}

func (c *PatientNotificationController) MarkAllRead(ctx *echo.Context) error {
	claims := getPatientClaimsFromCtx(ctx)

	data, err := c.UseCase.MarkAllRead(ctx.Request().Context(), claims.PatientID)
	if err != nil {
		return ctx.JSON(http.StatusInternalServerError, model.WebResponse[any]{Message: constants.MsgInternalServerError, Errors: err.Error()})
	}

	return ctx.JSON(http.StatusOK, model.WebResponse[*model.MarkAllReadResponse]{
		Message: "semua notifikasi ditandai sudah dibaca",
		Data:    data,
	})
}
