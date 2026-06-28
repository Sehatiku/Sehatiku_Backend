package usecase

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sehatiku-backend/internal/entity"
	"sehatiku-backend/internal/helper"
	"sehatiku-backend/internal/model"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

var ErrAssignedNakesNotFound = errors.New("dokter penanggung jawab tidak ditemukan")

type assignedNakesPatientRepo interface {
	FindByCondition(db *gorm.DB, condition string, args ...any) (*entity.Patient, error)
}

type assignedNakesNakesRepo interface {
	FindByID(db *gorm.DB, id string) (*entity.Nakes, error)
}

type AssignedNakesUseCase struct {
	DB          *gorm.DB
	PatientRepo assignedNakesPatientRepo
	NakesRepo   assignedNakesNakesRepo
	Log         *zap.Logger
}

func (u *AssignedNakesUseCase) GetAssignedNakes(ctx context.Context, patientID string) (*model.AssignedNakesResponse, error) {
	patient, err := u.PatientRepo.FindByCondition(u.DB, "id = ?", patientID)
	if err != nil {
		return nil, fmt.Errorf("loading patient %s: %w", patientID, err)
	}

	nakes, err := u.NakesRepo.FindByID(u.DB, patient.AssignedNakesID)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, ErrAssignedNakesNotFound
		}
		return nil, fmt.Errorf("loading assigned nakes %s: %w", patient.AssignedNakesID, err)
	}

	normalizedPhone := helper.NormalizePhoneID(nakes.PhoneNumber)
	resp := &model.AssignedNakesResponse{
		FullName:      nakes.FullName,
		WhatsappPhone: nakes.PhoneNumber,
		WaLink:        helper.BuildWAMeLink(normalizedPhone, ""),
		Schedule:      []model.ScheduleEntry{},
	}

	if nakes.Specialization != nil {
		resp.Specialization = *nakes.Specialization
	}
	if nakes.Hospital != nil {
		resp.Hospital = *nakes.Hospital
	}
	if nakes.Schedule != nil {
		var schedule []model.ScheduleEntry
		if err := json.Unmarshal([]byte(*nakes.Schedule), &schedule); err == nil {
			resp.Schedule = schedule
		} else {
			u.Log.Warn("failed to parse nakes schedule json",
				zap.String("nakes_id", nakes.ID),
				zap.Error(err),
			)
		}
	}

	return resp, nil
}
