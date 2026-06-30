package controller

import (
	"context"
	"errors"
	"net/http"
	"sehatiku-backend/internal/model"
	"sehatiku-backend/internal/usecase"
	"strconv"

	"github.com/labstack/echo/v5"
)

type escalationUseCase interface {
	GetQueue(ctx context.Context, faskesID, status, tier string, page, size int) ([]model.EscalationQueueItem, model.PageMetadata, error)
	View(ctx context.Context, id, faskesID string) error
	Act(ctx context.Context, id, faskesID string) error
	Dismiss(ctx context.Context, id, faskesID string) error
}

type EscalationController struct {
	UseCase escalationUseCase
}

func (c *EscalationController) GetQueue(ctx *echo.Context) error {
	claims := getNakesClaimsFromCtx(ctx)

	page, _ := strconv.Atoi(ctx.QueryParam("page"))
	size, _ := strconv.Atoi(ctx.QueryParam("size"))
	status := ctx.QueryParam("status")
	tier := ctx.QueryParam("tier")

	items, paging, err := c.UseCase.GetQueue(ctx.Request().Context(), claims.FaskesID, status, tier, page, size)
	if err != nil {
		return ctx.JSON(http.StatusInternalServerError, model.WebResponse[any]{
			Message: "internal server error",
			Errors:  err.Error(),
		})
	}
	return ctx.JSON(http.StatusOK, model.WebResponse[[]model.EscalationQueueItem]{
		Message: "antrian eskalasi berhasil diambil",
		Data:    items,
		Paging:  &paging,
	})
}

func (c *EscalationController) View(ctx *echo.Context) error    { return c.transition(ctx, "view") }
func (c *EscalationController) Act(ctx *echo.Context) error     { return c.transition(ctx, "act") }
func (c *EscalationController) Dismiss(ctx *echo.Context) error { return c.transition(ctx, "dismiss") }

func (c *EscalationController) transition(ctx *echo.Context, action string) error {
	claims := getNakesClaimsFromCtx(ctx)
	id := ctx.Param("id")
	if id == "" {
		return ctx.JSON(http.StatusBadRequest, model.WebResponse[any]{Message: "escalation id wajib diisi"})
	}

	var err error
	switch action {
	case "view":
		err = c.UseCase.View(ctx.Request().Context(), id, claims.FaskesID)
	case "act":
		err = c.UseCase.Act(ctx.Request().Context(), id, claims.FaskesID)
	case "dismiss":
		err = c.UseCase.Dismiss(ctx.Request().Context(), id, claims.FaskesID)
	}
	if err != nil {
		if errors.Is(err, usecase.ErrEscalationNotFound) {
			return ctx.JSON(http.StatusNotFound, model.WebResponse[any]{Message: "eskalasi tidak ditemukan", Errors: err.Error()})
		}
		if errors.Is(err, usecase.ErrEscalationClosed) {
			return ctx.JSON(http.StatusConflict, model.WebResponse[any]{Message: "eskalasi sudah ditutup", Errors: err.Error()})
		}
		return ctx.JSON(http.StatusInternalServerError, model.WebResponse[any]{Message: "internal server error", Errors: err.Error()})
	}
	return ctx.JSON(http.StatusOK, model.WebResponse[any]{Message: "status eskalasi berhasil diperbarui", Data: nil})
}
