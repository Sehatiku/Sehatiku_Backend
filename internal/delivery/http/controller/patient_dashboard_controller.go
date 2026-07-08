package controller

import (
	"context"
	"net/http"
	"sehatiku-backend/internal/constants"
	"sehatiku-backend/internal/model"

	"github.com/labstack/echo/v5"
)

type patientDashboardUseCase interface {
	GetDashboard(ctx context.Context, patientID string) (*model.PatientDashboardResponse, error)
}

type PatientDashboardController struct {
	UseCase patientDashboardUseCase
}

func (c *PatientDashboardController) GetDashboard(ctx *echo.Context) error {
	claims := getPatientClaimsFromCtx(ctx)

	data, err := c.UseCase.GetDashboard(ctx.Request().Context(), claims.PatientID)
	if err != nil {
		return ctx.JSON(http.StatusInternalServerError, model.WebResponse[any]{
			Message: constants.MsgInternalServerError,
			Errors:  err.Error(),
		})
	}

	return ctx.JSON(http.StatusOK, model.WebResponse[*model.PatientDashboardResponse]{
		Message: "dashboard pasien berhasil diambil",
		Data:    data,
	})
}

func getPatientClaimsFromCtx(ctx *echo.Context) *model.PatientAuthClaims {
	return ctx.Get("patient_auth").(*model.PatientAuthClaims)
}
