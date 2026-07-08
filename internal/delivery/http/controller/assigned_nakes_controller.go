package controller

import (
	"context"
	"errors"
	"net/http"
	"sehatiku-backend/internal/constants"
	"sehatiku-backend/internal/model"
	"sehatiku-backend/internal/usecase"

	"github.com/labstack/echo/v5"
)

type assignedNakesUseCase interface {
	GetAssignedNakes(ctx context.Context, patientID string) (*model.AssignedNakesResponse, error)
}

type AssignedNakesController struct {
	UseCase assignedNakesUseCase
}

func (c *AssignedNakesController) GetAssignedNakes(ctx *echo.Context) error {
	claims := getPatientClaimsFromCtx(ctx)

	data, err := c.UseCase.GetAssignedNakes(ctx.Request().Context(), claims.PatientID)
	if err != nil {
		if errors.Is(err, usecase.ErrAssignedNakesNotFound) {
			return ctx.JSON(http.StatusNotFound, model.WebResponse[any]{
				Message: constants.MsgNotFound,
				Errors:  err.Error(),
			})
		}
		return ctx.JSON(http.StatusInternalServerError, model.WebResponse[any]{
			Message: constants.MsgInternalServerError,
			Errors:  err.Error(),
		})
	}

	return ctx.JSON(http.StatusOK, model.WebResponse[*model.AssignedNakesResponse]{
		Message: "informasi dokter berhasil diambil",
		Data:    data,
	})
}
