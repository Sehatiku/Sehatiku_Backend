package usecase_test

import (
	"context"
	"sehatiku-backend/internal/entity"
	"sehatiku-backend/internal/model"
	"sehatiku-backend/internal/usecase"
	"testing"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

// ── Mocks ───────────────────────────────────────────────────────────────────

type mockEscalationRepo struct {
	active     *entity.Escalation
	activeErr  error
	created    []*entity.Escalation
	createErr  error
	byID       *entity.Escalation
	byIDErr    error
	updated    []struct{ id, status string }
	updateErr  error
	queueItems []model.EscalationQueueItem
	queueTotal int64
	queueErr   error
}

func (m *mockEscalationRepo) Create(_ *gorm.DB, e *entity.Escalation) error {
	if m.createErr != nil {
		return m.createErr
	}
	m.created = append(m.created, e)
	return nil
}
func (m *mockEscalationRepo) FindActiveByPatientTier(_ *gorm.DB, _, _ string) (*entity.Escalation, error) {
	return m.active, m.activeErr
}
func (m *mockEscalationRepo) FindByID(_ *gorm.DB, _ string) (*entity.Escalation, error) {
	return m.byID, m.byIDErr
}
func (m *mockEscalationRepo) UpdateStatus(_ *gorm.DB, id, status string, _ time.Time) error {
	if m.updateErr != nil {
		return m.updateErr
	}
	m.updated = append(m.updated, struct{ id, status string }{id, status})
	return nil
}
func (m *mockEscalationRepo) FindByFaskes(_ *gorm.DB, _, _, _ string, _, _ int) ([]model.EscalationQueueItem, int64, error) {
	return m.queueItems, m.queueTotal, m.queueErr
}

type mockRiskReader struct {
	status string
	found  bool
	err    error
}

func (m *mockRiskReader) FindLatestStatus(_ *gorm.DB, _, _ string) (string, bool, error) {
	return m.status, m.found, m.err
}

func newEscalationUC(repo *mockEscalationRepo, risk *mockRiskReader) *usecase.EscalationUseCase {
	return &usecase.EscalationUseCase{
		DB:       nil, // mocks ignore the db handle
		Repo:     repo,
		RiskRepo: risk,
		Log:      zap.NewNop(),
	}
}

func bahayaScore() *entity.RiskScore {
	return &entity.RiskScore{ID: "rs1", PatientID: "p1", Status: entity.RiskStatusBahaya}
}
func samplePatient() *entity.Patient {
	return &entity.Patient{ID: "p1", FaskesID: "f1", AssignedNakesID: "n1"}
}

// ── Acute evaluation tests ──────────────────────────────────────────────────

func TestEvaluateAcute_FiresOnTransitionToBahaya(t *testing.T) {
	repo := &mockEscalationRepo{}
	risk := &mockRiskReader{status: entity.RiskStatusWaswas, found: true}
	uc := newEscalationUC(repo, risk)

	if err := uc.EvaluateAcute(context.Background(), samplePatient(), bahayaScore()); err != nil {
		t.Fatalf("EvaluateAcute returned error: %v", err)
	}
	if len(repo.created) != 1 {
		t.Fatalf("expected 1 escalation created, got %d", len(repo.created))
	}
	got := repo.created[0]
	if got.Tier != entity.EscalationTierAcuteToday || got.Status != entity.EscalationStatusSent {
		t.Errorf("escalation tier/status = %s/%s; want acute_today/sent", got.Tier, got.Status)
	}
	if got.FaskesID != "f1" || got.AssignedNakesID != "n1" || got.RiskScoreID != "rs1" {
		t.Errorf("escalation FK mismatch: %+v", got)
	}
}

func TestEvaluateAcute_FiresWhenNoPriorScore(t *testing.T) {
	repo := &mockEscalationRepo{}
	risk := &mockRiskReader{found: false}
	uc := newEscalationUC(repo, risk)

	_ = uc.EvaluateAcute(context.Background(), samplePatient(), bahayaScore())
	if len(repo.created) != 1 {
		t.Fatalf("expected 1 escalation created on first-ever bahaya, got %d", len(repo.created))
	}
}

func TestEvaluateAcute_SkipsWhenAlreadyBahaya(t *testing.T) {
	repo := &mockEscalationRepo{}
	risk := &mockRiskReader{status: entity.RiskStatusBahaya, found: true}
	uc := newEscalationUC(repo, risk)

	_ = uc.EvaluateAcute(context.Background(), samplePatient(), bahayaScore())
	if len(repo.created) != 0 {
		t.Errorf("expected no escalation when already bahaya (no transition), got %d", len(repo.created))
	}
}

func TestEvaluateAcute_SkipsWhenNotBahaya(t *testing.T) {
	repo := &mockEscalationRepo{}
	uc := newEscalationUC(repo, &mockRiskReader{})

	score := &entity.RiskScore{ID: "rs1", PatientID: "p1", Status: entity.RiskStatusWaswas}
	_ = uc.EvaluateAcute(context.Background(), samplePatient(), score)
	if len(repo.created) != 0 {
		t.Errorf("expected no escalation for non-bahaya score, got %d", len(repo.created))
	}
}

func TestEvaluateAcute_SkipsWhenActiveExists(t *testing.T) {
	repo := &mockEscalationRepo{active: &entity.Escalation{ID: "e-existing", Status: entity.EscalationStatusSent}}
	risk := &mockRiskReader{status: entity.RiskStatusWaswas, found: true}
	uc := newEscalationUC(repo, risk)

	_ = uc.EvaluateAcute(context.Background(), samplePatient(), bahayaScore())
	if len(repo.created) != 0 {
		t.Errorf("expected dedup skip when an active escalation exists, got %d created", len(repo.created))
	}
}

// ── Lifecycle + queue tests ─────────────────────────────────────────────────

func TestView_MarksViewedWhenSent(t *testing.T) {
	repo := &mockEscalationRepo{byID: &entity.Escalation{ID: "e1", FaskesID: "f1", Status: entity.EscalationStatusSent}}
	uc := newEscalationUC(repo, &mockRiskReader{})

	if err := uc.View(context.Background(), "e1", "f1"); err != nil {
		t.Fatalf("View returned error: %v", err)
	}
	if len(repo.updated) != 1 || repo.updated[0].status != entity.EscalationStatusViewed {
		t.Errorf("expected one update to viewed, got %v", repo.updated)
	}
}

func TestView_IdempotentWhenAlreadyViewed(t *testing.T) {
	repo := &mockEscalationRepo{byID: &entity.Escalation{ID: "e1", FaskesID: "f1", Status: entity.EscalationStatusViewed}}
	uc := newEscalationUC(repo, &mockRiskReader{})

	if err := uc.View(context.Background(), "e1", "f1"); err != nil {
		t.Fatalf("View returned error: %v", err)
	}
	if len(repo.updated) != 0 {
		t.Errorf("expected no update when already viewed, got %v", repo.updated)
	}
}

func TestAct_MarksActed(t *testing.T) {
	repo := &mockEscalationRepo{byID: &entity.Escalation{ID: "e1", FaskesID: "f1", Status: entity.EscalationStatusViewed}}
	uc := newEscalationUC(repo, &mockRiskReader{})

	if err := uc.Act(context.Background(), "e1", "f1"); err != nil {
		t.Fatalf("Act returned error: %v", err)
	}
	if len(repo.updated) != 1 || repo.updated[0].status != entity.EscalationStatusActed {
		t.Errorf("expected one update to acted, got %v", repo.updated)
	}
}

func TestAct_ConflictWhenAlreadyClosed(t *testing.T) {
	repo := &mockEscalationRepo{byID: &entity.Escalation{ID: "e1", FaskesID: "f1", Status: entity.EscalationStatusActed}}
	uc := newEscalationUC(repo, &mockRiskReader{})

	err := uc.Act(context.Background(), "e1", "f1")
	if err != usecase.ErrEscalationClosed {
		t.Errorf("expected ErrEscalationClosed, got %v", err)
	}
	if len(repo.updated) != 0 {
		t.Errorf("expected no update on closed escalation, got %v", repo.updated)
	}
}

func TestDismiss_MarksDismissed(t *testing.T) {
	repo := &mockEscalationRepo{byID: &entity.Escalation{ID: "e1", FaskesID: "f1", Status: entity.EscalationStatusSent}}
	uc := newEscalationUC(repo, &mockRiskReader{})

	if err := uc.Dismiss(context.Background(), "e1", "f1"); err != nil {
		t.Fatalf("Dismiss returned error: %v", err)
	}
	if len(repo.updated) != 1 || repo.updated[0].status != entity.EscalationStatusDismissed {
		t.Errorf("expected one update to dismissed, got %v", repo.updated)
	}
}

func TestAct_ForeignFaskesIsNotFound(t *testing.T) {
	repo := &mockEscalationRepo{byID: &entity.Escalation{ID: "e1", FaskesID: "OTHER", Status: entity.EscalationStatusSent}}
	uc := newEscalationUC(repo, &mockRiskReader{})

	err := uc.Act(context.Background(), "e1", "f1")
	if err != usecase.ErrEscalationNotFound {
		t.Errorf("expected ErrEscalationNotFound for foreign faskes, got %v", err)
	}
}

func TestGetQueue_ReturnsItemsAndPaging(t *testing.T) {
	repo := &mockEscalationRepo{
		queueItems: []model.EscalationQueueItem{{ID: "e1", PatientName: "Budi"}},
		queueTotal: 1,
	}
	uc := newEscalationUC(repo, &mockRiskReader{})

	items, paging, err := uc.GetQueue(context.Background(), "f1", "", "", 1, 20)
	if err != nil {
		t.Fatalf("GetQueue returned error: %v", err)
	}
	if len(items) != 1 || items[0].PatientName != "Budi" {
		t.Errorf("unexpected items: %+v", items)
	}
	if paging.TotalItem != 1 || paging.TotalPage != 1 || paging.Page != 1 || paging.Size != 20 {
		t.Errorf("unexpected paging: %+v", paging)
	}
}
