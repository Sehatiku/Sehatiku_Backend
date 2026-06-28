package usecase

import (
	"context"
	"errors"
	"testing"

	"sehatiku-backend/internal/entity"
	"sehatiku-backend/internal/repository"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

// ── Mocks ────────────────────────────────────────────────────────────────────

type mockPendingStore struct {
	store     map[string]*repository.PendingCredential
	getErr    error
	deleted   []string
	deleteErr error
}

func (m *mockPendingStore) Get(_ context.Context, phone string) (*repository.PendingCredential, error) {
	if m.getErr != nil {
		return nil, m.getErr
	}
	return m.store[phone], nil
}

func (m *mockPendingStore) Delete(_ context.Context, phone string) error {
	m.deleted = append(m.deleted, phone)
	return m.deleteErr
}

type mockWarmupSender struct {
	patientCalls   []string
	companionCalls []string
	err            error
}

func (m *mockWarmupSender) SendRegistrationCredentials(_ context.Context, toPhone, _, _, _ string) error {
	m.patientCalls = append(m.patientCalls, toPhone)
	return m.err
}

func (m *mockWarmupSender) SendCompanionRegistrationCredentials(_ context.Context, toPhone, _, _, _, _ string) error {
	m.companionCalls = append(m.companionCalls, toPhone)
	return m.err
}

type mockNotifUpdater struct {
	statuses []string
}

func (m *mockNotifUpdater) MarkStatus(_ *gorm.DB, _, status string, _ *string) error {
	m.statuses = append(m.statuses, status)
	return nil
}

func newInboundUC(ps *mockPendingStore, wa *mockWarmupSender, nr *mockNotifUpdater) *WAInboundUseCase {
	return &WAInboundUseCase{
		DB:                nil,
		PendingCredential: ps,
		WhatsApp:          wa,
		NotificationRepo:  nr,
		Log:               zap.NewNop(),
	}
}

// ── Tests ────────────────────────────────────────────────────────────────────

func TestDeliverPendingCredential_Patient(t *testing.T) {
	ps := &mockPendingStore{store: map[string]*repository.PendingCredential{
		"628111": {Role: entity.RecipientRolePatient, RecipientName: "Budi", Username: "budi", Password: "secret12", NotificationID: "n1"},
	}}
	wa := &mockWarmupSender{}
	nr := &mockNotifUpdater{}
	uc := newInboundUC(ps, wa, nr)

	if err := uc.DeliverPendingCredential(context.Background(), []string{"628111"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(wa.patientCalls) != 1 || wa.patientCalls[0] != "628111" {
		t.Errorf("patientCalls = %v; want [628111]", wa.patientCalls)
	}
	if len(wa.companionCalls) != 0 {
		t.Errorf("companionCalls = %v; want none", wa.companionCalls)
	}
	if len(nr.statuses) != 1 || nr.statuses[0] != entity.NotificationStatusSent {
		t.Errorf("statuses = %v; want [sent]", nr.statuses)
	}
	if len(ps.deleted) != 1 || ps.deleted[0] != "628111" {
		t.Errorf("deleted = %v; want [628111]", ps.deleted)
	}
}

func TestDeliverPendingCredential_Companion(t *testing.T) {
	ps := &mockPendingStore{store: map[string]*repository.PendingCredential{
		"628222": {Role: entity.RecipientRoleCompanion, RecipientName: "Ibu Budi", PatientName: "Budi", Username: "budi", Password: "secret12", NotificationID: "n2"},
	}}
	wa := &mockWarmupSender{}
	uc := newInboundUC(ps, wa, &mockNotifUpdater{})

	if err := uc.DeliverPendingCredential(context.Background(), []string{"628222"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(wa.companionCalls) != 1 || wa.companionCalls[0] != "628222" {
		t.Errorf("companionCalls = %v; want [628222]", wa.companionCalls)
	}
	if len(wa.patientCalls) != 0 {
		t.Errorf("patientCalls = %v; want none", wa.patientCalls)
	}
}

func TestDeliverPendingCredential_NoPending(t *testing.T) {
	ps := &mockPendingStore{store: map[string]*repository.PendingCredential{}}
	wa := &mockWarmupSender{}
	nr := &mockNotifUpdater{}
	uc := newInboundUC(ps, wa, nr)

	if err := uc.DeliverPendingCredential(context.Background(), []string{"628999", "628000"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(wa.patientCalls)+len(wa.companionCalls) != 0 {
		t.Error("expected no sends when no pending credential")
	}
	if len(ps.deleted) != 0 {
		t.Error("expected no delete when no pending credential")
	}
	if len(nr.statuses) != 0 {
		t.Error("expected no notification update when no pending credential")
	}
}

func TestDeliverPendingCredential_SendFails_KeepsStash(t *testing.T) {
	ps := &mockPendingStore{store: map[string]*repository.PendingCredential{
		"628111": {Role: entity.RecipientRolePatient, RecipientName: "Budi", Username: "budi", Password: "secret12", NotificationID: "n1"},
	}}
	wa := &mockWarmupSender{err: errors.New("not connected")}
	nr := &mockNotifUpdater{}
	uc := newInboundUC(ps, wa, nr)

	err := uc.DeliverPendingCredential(context.Background(), []string{"628111"})
	if err == nil {
		t.Fatal("expected error when send fails")
	}
	if len(ps.deleted) != 0 {
		t.Errorf("stash must NOT be deleted on send failure, got deleted=%v", ps.deleted)
	}
	if len(nr.statuses) != 1 || nr.statuses[0] != entity.NotificationStatusFailed {
		t.Errorf("statuses = %v; want [failed]", nr.statuses)
	}
}

func TestDeliverPendingCredential_TriesAllCandidates(t *testing.T) {
	ps := &mockPendingStore{store: map[string]*repository.PendingCredential{
		// pending hanya cocok pada kandidat KEDUA (mis. nomor telepon di SenderAlt)
		"628333": {Role: entity.RecipientRolePatient, RecipientName: "Siti", Username: "siti", Password: "secret12", NotificationID: "n3"},
	}}
	wa := &mockWarmupSender{}
	uc := newInboundUC(ps, wa, &mockNotifUpdater{})

	if err := uc.DeliverPendingCredential(context.Background(), []string{"99lid", "628333"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(wa.patientCalls) != 1 || wa.patientCalls[0] != "628333" {
		t.Errorf("patientCalls = %v; want [628333]", wa.patientCalls)
	}
}
