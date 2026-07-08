package controller

import (
	"errors"
	"net/http"
	"sehatiku-backend/internal/constants"
	"sehatiku-backend/internal/gateway/gemini"
	"sehatiku-backend/internal/gateway/ml"
	"sehatiku-backend/internal/gateway/ocr"
	"sehatiku-backend/internal/model"
	"sehatiku-backend/internal/repository"
	"sehatiku-backend/internal/usecase"

	"github.com/labstack/echo/v5"
)

func mapAuthError(ctx *echo.Context, err error) error {
	switch {
	case errors.Is(err, usecase.ErrInvalidCredentials),
		errors.Is(err, usecase.ErrAccountInactive):
		return ctx.JSON(http.StatusUnauthorized, model.WebResponse[any]{
			Message: constants.MsgUnauthorized,
			Errors:  err.Error(),
		})
	case errors.Is(err, repository.ErrTooManyLoginAttempts):
		return ctx.JSON(http.StatusTooManyRequests, model.WebResponse[any]{
			Message: constants.MsgTooManyRequests,
			Errors:  err.Error(),
		})
	default:
		return ctx.JSON(http.StatusInternalServerError, model.WebResponse[any]{
			Message: constants.MsgInternalServerError,
			Errors:  err.Error(),
		})
	}
}

func mapFaskesError(ctx *echo.Context, err error) error {
	switch {
	case errors.Is(err, usecase.ErrFaskesNotFound):
		return ctx.JSON(http.StatusNotFound, model.WebResponse[any]{
			Message: constants.MsgNotFound,
			Errors:  err.Error(),
		})
	default:
		return ctx.JSON(http.StatusInternalServerError, model.WebResponse[any]{
			Message: constants.MsgInternalServerError,
			Errors:  err.Error(),
		})
	}
}

func mapNakesError(ctx *echo.Context, err error) error {
	switch {
	case errors.Is(err, usecase.ErrNakesNotFound):
		return ctx.JSON(http.StatusNotFound, model.WebResponse[any]{
			Message: constants.MsgNotFound,
			Errors:  err.Error(),
		})
	default:
		return ctx.JSON(http.StatusInternalServerError, model.WebResponse[any]{
			Message: constants.MsgInternalServerError,
			Errors:  err.Error(),
		})
	}
}

func mapPatientError(ctx *echo.Context, err error) error {
	switch {
	case errors.Is(err, usecase.ErrPatientNotFound):
		return ctx.JSON(http.StatusNotFound, model.WebResponse[any]{
			Message: constants.MsgNotFound,
			Errors:  err.Error(),
		})
	default:
		return ctx.JSON(http.StatusInternalServerError, model.WebResponse[any]{
			Message: constants.MsgInternalServerError,
			Errors:  err.Error(),
		})
	}
}

func mapBaselineError(ctx *echo.Context, err error) error {
	switch {
	case errors.Is(err, usecase.ErrPatientNotFound),
		errors.Is(err, usecase.ErrBaselineNotFound):
		return ctx.JSON(http.StatusNotFound, model.WebResponse[any]{
			Message: constants.MsgNotFound,
			Errors:  err.Error(),
		})
	case errors.Is(err, usecase.ErrAssignedNakesInvalid),
		errors.Is(err, usecase.ErrInvalidRecordedAt):
		return ctx.JSON(http.StatusBadRequest, model.WebResponse[any]{
			Message: constants.MsgBadRequest,
			Errors:  err.Error(),
		})
	default:
		return ctx.JSON(http.StatusInternalServerError, model.WebResponse[any]{
			Message: constants.MsgInternalServerError,
			Errors:  err.Error(),
		})
	}
}

func mapSummaryError(ctx *echo.Context, err error) error {
	switch {
	case errors.Is(err, usecase.ErrInvalidWindow):
		return ctx.JSON(http.StatusBadRequest, model.WebResponse[any]{
			Message: constants.MsgBadRequest,
			Errors:  err.Error(),
		})
	case errors.Is(err, usecase.ErrPatientNotFound):
		return ctx.JSON(http.StatusNotFound, model.WebResponse[any]{
			Message: constants.MsgNotFound,
			Errors:  err.Error(),
		})
	default:
		return ctx.JSON(http.StatusInternalServerError, model.WebResponse[any]{
			Message: constants.MsgInternalServerError,
			Errors:  err.Error(),
		})
	}
}

func mapHealthLogError(ctx *echo.Context, err error) error {
	switch {
	case errors.Is(err, usecase.ErrInvalidHealthLog):
		return ctx.JSON(http.StatusBadRequest, model.WebResponse[any]{
			Message: constants.MsgBadRequest,
			Errors:  err.Error(),
		})
	case errors.Is(err, usecase.ErrIdempotencyInFlight):
		return ctx.JSON(http.StatusConflict, model.WebResponse[any]{
			Message: constants.MsgConflict,
			Errors:  err.Error(),
		})
	case errors.Is(err, usecase.ErrTooManySubmissions):
		return ctx.JSON(http.StatusTooManyRequests, model.WebResponse[any]{
			Message: constants.MsgTooManyRequests,
			Errors:  err.Error(),
		})
	default:
		return ctx.JSON(http.StatusInternalServerError, model.WebResponse[any]{
			Message: constants.MsgInternalServerError,
			Errors:  err.Error(),
		})
	}
}

func mapHealthScoreError(ctx *echo.Context, err error) error {
	switch {
	case errors.Is(err, usecase.ErrNoBaseline):
		return ctx.JSON(http.StatusUnprocessableEntity, model.WebResponse[any]{
			Message: constants.MsgUnprocessableEntity,
			Errors:  "baseline klinis pasien belum tersedia — belum bisa dihitung",
		})
	case errors.Is(err, ml.ErrMLUpstream):
		return ctx.JSON(http.StatusServiceUnavailable, model.WebResponse[any]{
			Message: constants.MsgServiceUnavailable,
			Errors:  err.Error(),
		})
	case errors.Is(err, ml.ErrMLUnauthorized), errors.Is(err, ml.ErrMLBadRequest):
		return ctx.JSON(http.StatusBadGateway, model.WebResponse[any]{
			Message: constants.MsgBadGateway,
			Errors:  err.Error(),
		})
	default:
		return ctx.JSON(http.StatusInternalServerError, model.WebResponse[any]{
			Message: constants.MsgInternalServerError,
			Errors:  err.Error(),
		})
	}
}

func mapRegistrationError(ctx *echo.Context, err error) error {
	switch {
	case errors.Is(err, usecase.ErrNIKAlreadyExists),
		errors.Is(err, usecase.ErrUsernameAlreadyExists),
		errors.Is(err, usecase.ErrPhoneAlreadyExists):
		return ctx.JSON(http.StatusConflict, model.WebResponse[any]{
			Message: constants.MsgConflict,
			Errors:  err.Error(),
		})
	case errors.Is(err, ocr.ErrOCRKTPUnreadable):
		return ctx.JSON(http.StatusUnprocessableEntity, model.WebResponse[any]{
			Message: constants.MsgUnprocessableEntity,
			Errors:  err.Error(),
		})
	case errors.Is(err, ocr.ErrOCRBadRequest),
		errors.Is(err, usecase.ErrAssignedNakesInvalid):
		return ctx.JSON(http.StatusBadRequest, model.WebResponse[any]{
			Message: constants.MsgBadRequest,
			Errors:  err.Error(),
		})
	case errors.Is(err, gemini.ErrGeminiBadRequest),
		errors.Is(err, gemini.ErrGeminiEmpty):
		// Dokumen tak terbaca / ekstraksi gagal → minta faskes unggah dokumen lebih jelas.
		return ctx.JSON(http.StatusUnprocessableEntity, model.WebResponse[any]{
			Message: constants.MsgUnprocessableEntity,
			Errors:  "dokumen template baseline tidak terbaca, coba unggah scan/foto yang lebih jelas",
		})
	case errors.Is(err, ocr.ErrOCRUnauthorized),
		errors.Is(err, ocr.ErrOCRUpstream),
		errors.Is(err, gemini.ErrGeminiUnauthorized),
		errors.Is(err, gemini.ErrGeminiUpstream):
		return ctx.JSON(http.StatusBadGateway, model.WebResponse[any]{
			Message: constants.MsgBadGateway,
			Errors:  err.Error(),
		})
	default:
		return ctx.JSON(http.StatusInternalServerError, model.WebResponse[any]{
			Message: constants.MsgInternalServerError,
			Errors:  err.Error(),
		})
	}
}
