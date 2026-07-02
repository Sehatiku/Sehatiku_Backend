package usecase_test

import (
	"context"
	"testing"
	"time"

	"sehatiku-backend/internal/entity"
	"sehatiku-backend/internal/model"
	"sehatiku-backend/internal/usecase"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

type mockConsultationRepo struct {
	byID    *entity.Consultation
	replied bool
}

func (m *mockConsultationRepo) Create(_ *gorm.DB, _ *entity.Consultation) error { return nil }
func (m *mockConsultationRepo) FindByPatientID(_ *gorm.DB, _ string) ([]entity.Consultation, error) {
	return nil, nil
}
func (m *mockConsultationRepo) FindByNakesID(_ *gorm.DB, _, _ string) ([]model.NakesConsultationItem, error) {
	return nil, nil
}
func (m *mockConsultationRepo) FindByID(_ *gorm.DB, _ string) (*entity.Consultation, error) {
	return m.byID, nil
}
func (m *mockConsultationRepo) Reply(_ *gorm.DB, _, _, _ string, _ time.Time) error {
	m.replied = true
	return nil
}

type mockConsultationNakesRepo struct{}

func (m *mockConsultationNakesRepo) FindByID(_ *gorm.DB, id string) (*entity.Nakes, error) {
	return &entity.Nakes{ID: id, FullName: "dr. Test"}, nil
}

type mockConsultationInboxRepo struct {
	created []*entity.PatientNotification
}

func (m *mockConsultationInboxRepo) Create(_ *gorm.DB, n *entity.PatientNotification) error {
	m.created = append(m.created, n)
	return nil
}

type mockConsultationPushNotifier struct {
	calls []struct{ patientID, title, body string }
	done  chan struct{} // signaled once Notify has recorded its call; lazily initialized
}

func (m *mockConsultationPushNotifier) Notify(_ context.Context, patientID, title, body string, _ map[string]string) {
	m.calls = append(m.calls, struct{ patientID, title, body string }{patientID, title, body})
	if m.done != nil {
		close(m.done)
	}
}

func TestConsultationUseCase_ReplyConsultation_NotifiesPush(t *testing.T) {
	repo := &mockConsultationRepo{
		byID: &entity.Consultation{ID: "c-1", PatientID: "patient-1", Status: entity.ConsultationStatusOpen},
	}
	push := &mockConsultationPushNotifier{done: make(chan struct{})}
	u := &usecase.ConsultationUseCase{
		Repo:      repo,
		NakesRepo: &mockConsultationNakesRepo{},
		InboxRepo: &mockConsultationInboxRepo{},
		Push:      push,
		Log:       zap.NewNop(),
	}

	req := &model.ReplyConsultationRequest{NakesNote: "Minum obat teratur ya"}
	if err := u.ReplyConsultation(context.Background(), "c-1", "nakes-1", req); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if !repo.replied {
		t.Fatal("expected consultation to be marked replied")
	}

	// Push notify now runs fire-and-forget in a goroutine (see consultation_usecase.go),
	// so wait deterministically for it to complete instead of racing on push.calls.
	select {
	case <-push.done:
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for push notify goroutine")
	}

	if len(push.calls) != 1 {
		t.Fatalf("expected push notify called once, got %d", len(push.calls))
	}
	if push.calls[0].patientID != "patient-1" {
		t.Fatalf("expected push to patient-1, got %s", push.calls[0].patientID)
	}
	if push.calls[0].title != "Balasan dari dokter" {
		t.Fatalf("expected title 'Balasan dari dokter', got %q", push.calls[0].title)
	}
}
