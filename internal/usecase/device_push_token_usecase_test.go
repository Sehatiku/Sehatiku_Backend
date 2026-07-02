// internal/usecase/device_push_token_usecase_test.go
package usecase_test

import (
	"context"
	"errors"
	"testing"

	"sehatiku-backend/internal/usecase"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

type mockDeviceTokenRepo struct {
	upsertCalls     []struct{ patientID, platform, token string }
	upsertErr       error
	deactivateCalls []struct{ patientID, token string }
	deactivateErr   error
}

func (m *mockDeviceTokenRepo) Upsert(_ *gorm.DB, patientID, platform, token string) error {
	if m.upsertErr != nil {
		return m.upsertErr
	}
	m.upsertCalls = append(m.upsertCalls, struct{ patientID, platform, token string }{patientID, platform, token})
	return nil
}
func (m *mockDeviceTokenRepo) DeactivateByToken(_ *gorm.DB, patientID, token string) error {
	if m.deactivateErr != nil {
		return m.deactivateErr
	}
	m.deactivateCalls = append(m.deactivateCalls, struct{ patientID, token string }{patientID, token})
	return nil
}

func TestDevicePushTokenUseCase_Register_CallsUpsert(t *testing.T) {
	repo := &mockDeviceTokenRepo{}
	u := &usecase.DevicePushTokenUseCase{Repo: repo, Log: zap.NewNop()}

	if err := u.Register(context.Background(), "patient-1", "tok-abc", "android"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(repo.upsertCalls) != 1 {
		t.Fatalf("expected 1 upsert call, got %d", len(repo.upsertCalls))
	}
	got := repo.upsertCalls[0]
	if got.patientID != "patient-1" || got.platform != "android" || got.token != "tok-abc" {
		t.Fatalf("unexpected upsert args: %+v", got)
	}
}

func TestDevicePushTokenUseCase_Register_RepoError_Propagates(t *testing.T) {
	repo := &mockDeviceTokenRepo{upsertErr: errors.New("db down")}
	u := &usecase.DevicePushTokenUseCase{Repo: repo, Log: zap.NewNop()}

	if err := u.Register(context.Background(), "patient-1", "tok-abc", "android"); err == nil {
		t.Fatal("expected error to propagate, got nil")
	}
}

func TestDevicePushTokenUseCase_Deregister_CallsDeactivate(t *testing.T) {
	repo := &mockDeviceTokenRepo{}
	u := &usecase.DevicePushTokenUseCase{Repo: repo, Log: zap.NewNop()}

	if err := u.Deregister(context.Background(), "patient-1", "tok-abc"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(repo.deactivateCalls) != 1 {
		t.Fatalf("expected 1 deactivate call, got %d", len(repo.deactivateCalls))
	}
	got := repo.deactivateCalls[0]
	if got.patientID != "patient-1" || got.token != "tok-abc" {
		t.Fatalf("unexpected deactivate args: %+v", got)
	}
}
