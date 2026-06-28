package controller

import (
	"context"
	"net/http"
	"sehatiku-backend/internal/model"

	"github.com/labstack/echo/v5"
)

type consultationUseCase interface {
	CreateConsultation(ctx context.Context, patientID string, req *model.CreateConsultationRequest) (*model.ConsultationResponse, error)
}

type ConsultationController struct {
	UseCase consultationUseCase
}

func (c *ConsultationController) Create(ctx *echo.Context) error {
	claims := getPatientClaimsFromCtx(ctx)

	req := new(model.CreateConsultationRequest)
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

	data, err := c.UseCase.CreateConsultation(ctx.Request().Context(), claims.PatientID, req)
	if err != nil {
		return ctx.JSON(http.StatusInternalServerError, model.WebResponse[any]{
			Message: "internal server error",
			Errors:  err.Error(),
		})
	}

	return ctx.JSON(http.StatusCreated, model.WebResponse[*model.ConsultationResponse]{
		Message: "keluhan berhasil dikirim",
		Data:    data,
	})
}
