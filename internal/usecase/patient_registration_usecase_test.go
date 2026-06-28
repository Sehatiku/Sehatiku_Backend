package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"sehatiku-backend/internal/entity"
	"sehatiku-backend/internal/gateway/whatsapp"
	"sehatiku-backend/internal/model"
	"sehatiku-backend/internal/repository"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

// ── Mocks ────────────────────────────────────────────────────────────────────

type mockPatientRepo struct {
	created *entity.Patient
}

func (m *mockPatientRepo) FindByNIK(_ *gorm.DB, _ string) (*entity.Patient, error) {
	return nil, gorm.ErrRecordNotFound
}
func (m *mockPatientRepo) FindByUsername(_ *gorm.DB, _ string) (*entity.Patient, error) {
	return nil, gorm.ErrRecordNotFound
}
func (m *mockPatientRepo) Create(_ *gorm.DB, p *entity.Patient) error {
	p.ID = "patient-1"
	m.created = p
	return nil
}

type mockRegNakesRepo struct{ faskesID string }

func (m *mockRegNakesRepo) FindByID(_ *gorm.DB, id string) (*entity.Nakes, error) {
	return &entity.Nakes{ID: id, FaskesID: m.faskesID}, nil
}

type mockRegNotifRepo struct{ created int }

func (m *mockRegNotifRepo) Create(_ *gorm.DB, _ *entity.Notification) error {
	m.created++
	return nil
}

type mockStasher struct {
	stashed map[string]repository.PendingCredential
	err     error
}

func (m *mockStasher) Stash(_ context.Context, phone string, data repository.PendingCredential, _ time.Duration) error {
	if m.err != nil {
		return m.err
	}
	if m.stashed == nil {
		m.stashed = map[string]repository.PendingCredential{}
	}
	m.stashed[phone] = data
	return nil
}

func newPatientRegUC(stasher pendingCredentialStasher, notif notificationRepo) *PatientRegistrationUseCase {
	return &PatientRegistrationUseCase{
		DB:                nil,
		PatientRepo:       &mockPatientRepo{},
		NakesRepo:         &mockRegNakesRepo{faskesID: "faskes-1"},
		NotificationRepo:  notif,
		PendingCredential: stasher,
		WhatsApp:          &whatsapp.WhatsAppGateway{}, // Client nil → BotPhone() == "" (bot belum paired)
		Log:               zap.NewNop(),
	}
}

func validPatientReq() *model.PatientRegisterRequest {
	return &model.PatientRegisterRequest{
		AssignedNakesID: "nakes-1",
		NIK:             "1234567890123456",
		FullName:        "Budi",
		DateOfBirth:     "1960-01-02",
		Sex:             "male",
		Alamat:          "Jl. Mawar",
		PhoneNumber:     "081111111111",
		CompanionName:   "Ibu Budi",
		CompanionPhone:  "082222222222",
		DiseaseType:     "diabetes_t2",
		Username:        "budi",
		Password:        "secret12",
	}
}

// ── Tests ────────────────────────────────────────────────────────────────────

func TestRegisterPatient_StashesWarmupAndReturnsCredentials(t *testing.T) {
	stasher := &mockStasher{}
	notif := &mockRegNotifRepo{}
	uc := newPatientRegUC(stasher, notif)

	resp, err := uc.RegisterPatient(context.Background(), "faskes-1", validPatientReq())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Kredensial dikembalikan sebagai kanal cadangan terjamin.
	if resp.Credentials.Username != "budi" || resp.Credentials.Password != "secret12" {
		t.Errorf("credentials = %+v; want username=budi password=secret12", resp.Credentials)
	}

	// Pasien & pendamping di-stash untuk warm-up.
	if len(stasher.stashed) != 2 {
		t.Fatalf("stashed %d entries; want 2 (patient + companion)", len(stasher.stashed))
	}
	if got := stasher.stashed["081111111111"].Role; got != entity.RecipientRolePatient {
		t.Errorf("patient stash role = %q; want patient", got)
	}
	if got := stasher.stashed["082222222222"].Role; got != entity.RecipientRoleCompanion {
		t.Errorf("companion stash role = %q; want companion", got)
	}
	if stasher.stashed["082222222222"].PatientName != "Budi" {
		t.Errorf("companion stash missing patient name")
	}

	// Bot belum paired → status unavailable, link kosong.
	if resp.WAWarmup.Status != warmupStatusUnavailable {
		t.Errorf("warmup status = %q; want unavailable", resp.WAWarmup.Status)
	}
	if resp.WAWarmup.PatientLink != "" {
		t.Errorf("expected empty patient link when bot not paired, got %q", resp.WAWarmup.PatientLink)
	}
	// Tanpa link, tidak ada pesan siap-bagikan yang bisa ditindaklanjuti.
	if resp.WAWarmup.PatientMessage != "" {
		t.Errorf("expected empty patient message when bot not paired, got %q", resp.WAWarmup.PatientMessage)
	}
	if resp.WAWarmup.CompanionMessage != "" {
		t.Errorf("expected empty companion message when bot not paired, got %q", resp.WAWarmup.CompanionMessage)
	}

	// Dua baris audit notifikasi tercatat.
	if notif.created != 2 {
		t.Errorf("notifications created = %d; want 2", notif.created)
	}
}

func TestRegisterPatient_StashFailureDoesNotFailRegistration(t *testing.T) {
	stasher := &mockStasher{err: errors.New("redis down")}
	uc := newPatientRegUC(stasher, &mockRegNotifRepo{})

	resp, err := uc.RegisterPatient(context.Background(), "faskes-1", validPatientReq())
	if err != nil {
		t.Fatalf("stash failure must not fail registration, got error: %v", err)
	}
	if resp.PatientID == "" {
		t.Error("expected patient to be created despite stash failure")
	}
	if resp.Credentials.Password != "secret12" {
		t.Error("credentials must still be returned despite stash failure")
	}
}
