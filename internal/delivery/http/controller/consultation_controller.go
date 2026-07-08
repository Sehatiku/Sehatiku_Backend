package controller

import (
	"context"
	"net/http"
	"sehatiku-backend/internal/constants"
	"sehatiku-backend/internal/model"
	"strings"

	"github.com/labstack/echo/v5"
)

type consultationUseCase interface {
	CreateConsultation(ctx context.Context, patientID string, req *model.CreateConsultationRequest) (*model.ConsultationResponse, error)
	GetPatientConsultations(ctx context.Context, patientID string) ([]model.ConsultationResponse, error)
	GetNakesConsultations(ctx context.Context, nakesID, faksesID string) ([]model.NakesConsultationItem, error)
	ReplyConsultation(ctx context.Context, consultationID, nakesID string, req *model.ReplyConsultationRequest) error
}

type ConsultationController struct {
	UseCase consultationUseCase
}

func (c *ConsultationController) Create(ctx *echo.Context) error {
	claims := getPatientClaimsFromCtx(ctx)

	req := new(model.CreateConsultationRequest)
	if err := ctx.Bind(req); err != nil {
		return ctx.JSON(http.StatusBadRequest, model.WebResponse[any]{Message: constants.MsgBadRequest, Errors: err.Error()})
	}
	if err := ctx.Validate(req); err != nil {
		return ctx.JSON(http.StatusBadRequest, model.WebResponse[any]{Message: constants.MsgValidationError, Errors: err.Error()})
	}

	data, err := c.UseCase.CreateConsultation(ctx.Request().Context(), claims.PatientID, req)
	if err != nil {
		return ctx.JSON(http.StatusInternalServerError, model.WebResponse[any]{Message: constants.MsgInternalServerError, Errors: err.Error()})
	}

	return ctx.JSON(http.StatusCreated, model.WebResponse[*model.ConsultationResponse]{
		Message: "keluhan berhasil dikirim",
		Data:    data,
	})
}

func (c *ConsultationController) GetPatientList(ctx *echo.Context) error {
	claims := getPatientClaimsFromCtx(ctx)

	data, err := c.UseCase.GetPatientConsultations(ctx.Request().Context(), claims.PatientID)
	if err != nil {
		return ctx.JSON(http.StatusInternalServerError, model.WebResponse[any]{Message: constants.MsgInternalServerError, Errors: err.Error()})
	}

	return ctx.JSON(http.StatusOK, model.WebResponse[[]model.ConsultationResponse]{
		Message: "daftar konsultasi berhasil diambil",
		Data:    data,
	})
}

func (c *ConsultationController) GetNakesList(ctx *echo.Context) error {
	claims := getNakesClaimsFromCtx(ctx)

	data, err := c.UseCase.GetNakesConsultations(ctx.Request().Context(), claims.NakesID, claims.FaskesID)
	if err != nil {
		return ctx.JSON(http.StatusInternalServerError, model.WebResponse[any]{Message: constants.MsgInternalServerError, Errors: err.Error()})
	}

	return ctx.JSON(http.StatusOK, model.WebResponse[[]model.NakesConsultationItem]{
		Message: "daftar konsultasi pasien berhasil diambil",
		Data:    data,
	})
}

func (c *ConsultationController) Reply(ctx *echo.Context) error {
	claims := getNakesClaimsFromCtx(ctx)
	consultationID := ctx.Param("id")
	if consultationID == "" {
		return ctx.JSON(http.StatusBadRequest, model.WebResponse[any]{Message: "consultation id wajib diisi"})
	}

	req := new(model.ReplyConsultationRequest)
	if err := ctx.Bind(req); err != nil {
		return ctx.JSON(http.StatusBadRequest, model.WebResponse[any]{Message: constants.MsgBadRequest, Errors: err.Error()})
	}
	if err := ctx.Validate(req); err != nil {
		return ctx.JSON(http.StatusBadRequest, model.WebResponse[any]{Message: constants.MsgValidationError, Errors: err.Error()})
	}

	if err := c.UseCase.ReplyConsultation(ctx.Request().Context(), consultationID, claims.NakesID, req); err != nil {
		if strings.Contains(err.Error(), "already replied") {
			return ctx.JSON(http.StatusConflict, model.WebResponse[any]{Message: "konsultasi sudah dibalas", Errors: err.Error()})
		}
		if strings.Contains(err.Error(), "not found") {
			return ctx.JSON(http.StatusNotFound, model.WebResponse[any]{Message: "konsultasi tidak ditemukan", Errors: err.Error()})
		}
		return ctx.JSON(http.StatusInternalServerError, model.WebResponse[any]{Message: constants.MsgInternalServerError, Errors: err.Error()})
	}

	return ctx.JSON(http.StatusOK, model.WebResponse[any]{
		Message: "balasan berhasil dikirim",
		Data:    nil,
	})
}
