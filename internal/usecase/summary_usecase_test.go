package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"sehatiku-backend/internal/entity"
	"sehatiku-backend/internal/repository"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

// --- mocks ---

type mockSummaryRepo struct {
	earliest *time.Time
	logDates []time.Time
}

func (m *mockSummaryRepo) GetWindowAggregates(_ *gorm.DB, _ string, _ time.Time) (*repository.WindowAggregatesRaw, error) {
	return &repository.WindowAggregatesRaw{}, nil
}
func (m *mockSummaryRepo) GetWeightWindow(_ *gorm.DB, _ string, _ time.Time) (*repository.WeightWindowRaw, error) {
	return &repository.WeightWindowRaw{}, nil
}
func (m *mockSummaryRepo) GetEarliestLogDate(_ *gorm.DB, _ string) (*time.Time, error) {
	return m.earliest, nil
}
func (m *mockSummaryRepo) GetLogDatesSince(_ *gorm.DB, _ string, _ time.Time) ([]time.Time, error) {
	return m.logDates, nil
}
func (m *mockSummaryRepo) GetLatestRisk(_ *gorm.DB, _ string) (*repository.SummaryRiskRow, error) {
	return nil, nil
}

type mockSummaryGenerator struct {
	calls int
	text  string
	err   error
}

func (m *mockSummaryGenerator) GenerateSummary(_ context.Context, _ string) (string, error) {
	m.calls++
	return m.text, m.err
}

type mockSummaryPatientRepo struct {
	patient *entity.Patient
	err     error
}

func (m *mockSummaryPatientRepo) FindByID(_ *gorm.DB, _ string) (*entity.Patient, error) {
	return m.patient, m.err
}

// earliestForSpan mengembalikan tanggal log pertama agar rentang riwayat = spanDays
// (inklusif hari pertama & hari ini WIB).
func earliestForSpan(spanDays int) *time.Time {
	today := truncateToDay(time.Now().In(wibLocation))
	d := today.AddDate(0, 0, -(spanDays - 1))
	return &d
}

func TestSummaryAvailabilityGate(t *testing.T) {
	tests := []struct {
		name          string
		spanDays      int
		window        int
		wantAvailable bool
		wantWindows   []int
		wantGenCalls  int
	}{
		{"riwayat 5 hari minta 7", 5, 7, false, []int{}, 0},
		{"riwayat 7 hari minta 7", 7, 7, true, []int{7}, 1},
		{"riwayat 10 hari minta 14", 10, 14, false, []int{7}, 0},
		{"riwayat 14 hari minta 14", 14, 14, true, []int{7, 14}, 1},
		{"riwayat 40 hari minta 30", 40, 30, true, []int{7, 14, 30}, 1},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			gen := &mockSummaryGenerator{text: "narasi ringkas"}
			uc := &SummaryUseCase{
				Repo:      &mockSummaryRepo{earliest: earliestForSpan(tt.spanDays)},
				Generator: gen,
				Log:       zap.NewNop(),
			}

			resp, err := uc.GetPatientSummary(context.Background(), "patient-1", tt.window)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if resp.Available != tt.wantAvailable {
				t.Errorf("Available = %v, want %v", resp.Available, tt.wantAvailable)
			}
			if !equalIntSlice(resp.AvailableWindows, tt.wantWindows) {
				t.Errorf("AvailableWindows = %v, want %v", resp.AvailableWindows, tt.wantWindows)
			}
			if gen.calls != tt.wantGenCalls {
				t.Errorf("generator calls = %d, want %d", gen.calls, tt.wantGenCalls)
			}
			if tt.wantAvailable && resp.Narrative != "narasi ringkas" {
				t.Errorf("Narrative = %q, want %q", resp.Narrative, "narasi ringkas")
			}
		})
	}
}

func TestSummaryInvalidWindow(t *testing.T) {
	uc := &SummaryUseCase{
		Repo:      &mockSummaryRepo{earliest: earliestForSpan(40)},
		Generator: &mockSummaryGenerator{},
		Log:       zap.NewNop(),
	}
	if _, err := uc.GetPatientSummary(context.Background(), "patient-1", 10); !errors.Is(err, ErrInvalidWindow) {
		t.Fatalf("expected ErrInvalidWindow, got %v", err)
	}
}

func TestNakesSummaryTenancy(t *testing.T) {
	gen := &mockSummaryGenerator{text: "x"}
	uc := &SummaryUseCase{
		Repo:        &mockSummaryRepo{earliest: earliestForSpan(40)},
		PatientRepo: &mockSummaryPatientRepo{patient: &entity.Patient{ID: "p1", FaskesID: "faskes-OTHER"}},
		Generator:   gen,
		Log:         zap.NewNop(),
	}

	_, err := uc.GetNakesPatientSummary(context.Background(), "faskes-MINE", "p1", 7)
	if !errors.Is(err, ErrPatientNotFound) {
		t.Fatalf("expected ErrPatientNotFound for cross-tenant access, got %v", err)
	}
	if gen.calls != 0 {
		t.Errorf("generator must not be called on tenancy rejection, calls = %d", gen.calls)
	}
}

func TestSummaryGeminiFailureFallback(t *testing.T) {
	gen := &mockSummaryGenerator{err: errors.New("gemini down")}
	uc := &SummaryUseCase{
		Repo:      &mockSummaryRepo{earliest: earliestForSpan(10)},
		Generator: gen,
		Log:       zap.NewNop(),
	}

	resp, err := uc.GetPatientSummary(context.Background(), "patient-1", 7)
	if err != nil {
		t.Fatalf("expected graceful degradation (nil error), got %v", err)
	}
	if !resp.Available {
		t.Errorf("Available = false, want true (aggregates still served)")
	}
	if resp.Narrative != summaryFallbackNarrative {
		t.Errorf("Narrative = %q, want fallback %q", resp.Narrative, summaryFallbackNarrative)
	}
}

func equalIntSlice(a, b []int) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
