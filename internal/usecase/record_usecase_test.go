package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"sehatiku-backend/internal/repository"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

// mockRecordHistoryRepo memenuhi interface recordHistoryRepo untuk menguji RecordUseCase
// tanpa menyentuh database.
type mockRecordHistoryRepo struct {
	lastAt *time.Time
	rows   []repository.RecordHistoryRaw
	err    error
}

func (m *mockRecordHistoryRepo) GetRecordHistory(_ *gorm.DB, _ string, _ int) ([]repository.RecordHistoryRaw, error) {
	return m.rows, m.err
}

func (m *mockRecordHistoryRepo) GetLastLogAt(_ *gorm.DB, _ string) (*time.Time, error) {
	return m.lastAt, m.err
}

func ptrTime(t time.Time) *time.Time { return &t }

func TestGetTodayStatus(t *testing.T) {
	now := time.Now().In(wibLocation)
	wantDate := now.Format("2006-01-02")

	tests := []struct {
		name            string
		lastAt          *time.Time
		wantLoggedToday bool
		wantDays        *int // nil = expect days_since_last_log null
	}{
		{
			name:            "belum pernah isi",
			lastAt:          nil,
			wantLoggedToday: false,
			wantDays:        nil,
		},
		{
			name:            "sudah isi hari ini",
			lastAt:          ptrTime(now),
			wantLoggedToday: true,
			wantDays:        intPtr(0),
		},
		{
			name:            "lupa 1 hari (terakhir kemarin)",
			lastAt:          ptrTime(now.AddDate(0, 0, -1)),
			wantLoggedToday: false,
			wantDays:        intPtr(1),
		},
		{
			name:            "lupa beberapa hari (terakhir 3 hari lalu)",
			lastAt:          ptrTime(now.AddDate(0, 0, -3)),
			wantLoggedToday: false,
			wantDays:        intPtr(3),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			uc := &RecordUseCase{
				HistoryRepo: &mockRecordHistoryRepo{lastAt: tt.lastAt},
				Log:         zap.NewNop(),
			}

			resp, err := uc.GetTodayStatus(context.Background(), "patient-1")
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if resp.LoggedToday != tt.wantLoggedToday {
				t.Errorf("LoggedToday = %v, want %v", resp.LoggedToday, tt.wantLoggedToday)
			}
			if resp.Date != wantDate {
				t.Errorf("Date = %q, want %q (WIB)", resp.Date, wantDate)
			}
			switch {
			case tt.wantDays == nil && resp.DaysSinceLastLog != nil:
				t.Errorf("DaysSinceLastLog = %v, want nil", *resp.DaysSinceLastLog)
			case tt.wantDays != nil && resp.DaysSinceLastLog == nil:
				t.Errorf("DaysSinceLastLog = nil, want %d", *tt.wantDays)
			case tt.wantDays != nil && *resp.DaysSinceLastLog != *tt.wantDays:
				t.Errorf("DaysSinceLastLog = %d, want %d", *resp.DaysSinceLastLog, *tt.wantDays)
			}
		})
	}
}

func TestGetTodayStatus_RepoError(t *testing.T) {
	uc := &RecordUseCase{
		HistoryRepo: &mockRecordHistoryRepo{err: errors.New("db down")},
		Log:         zap.NewNop(),
	}

	if _, err := uc.GetTodayStatus(context.Background(), "patient-1"); err == nil {
		t.Fatal("expected error when repo fails, got nil")
	}
}

func TestGetHistory_IncludesHealthScoreWhenAvailable(t *testing.T) {
	score := 82
	glucose := 75.0
	weight := 64.0
	uc := &RecordUseCase{
		HistoryRepo: &mockRecordHistoryRepo{
			rows: []repository.RecordHistoryRaw{
				{
					LogDate:     time.Date(2026, 6, 30, 0, 0, 0, 0, time.UTC),
					BloodSugar:  &glucose,
					Weight:      &weight,
					HealthScore: &score,
				},
			},
		},
		Log: zap.NewNop(),
	}

	items, err := uc.GetHistory(context.Background(), "patient-1", 7)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 1 {
		t.Fatalf("len(items) = %d, want 1", len(items))
	}
	if items[0].HealthScore == nil {
		t.Fatal("HealthScore = nil, want 82")
	}
	if *items[0].HealthScore != score {
		t.Errorf("HealthScore = %d, want %d", *items[0].HealthScore, score)
	}
}

func intPtr(i int) *int { return &i }
