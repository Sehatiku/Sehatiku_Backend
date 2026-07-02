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
	blocked    bool
	blockedErr error
	created    []*entity.Escalation
	createErr  error
	byID       *entity.Escalation
	byIDErr    error
	updated    []struct{ id, status string }
	updateErr  error
	queueItems []model.EscalationQueueItem
	queueTotal int64
	queueErr   error

	feedbackSet   []struct{ id, feedback string }
	feedbackErr   error
	todayCount    int64
	todayCountErr error
}

func (m *mockEscalationRepo) Create(_ *gorm.DB, e *entity.Escalation) error {
	if m.createErr != nil {
		return m.createErr
	}
	m.created = append(m.created, e)
	return nil
}
func (m *mockEscalationRepo) ExistsActiveOrRecent(_ *gorm.DB, _, _ string, _ time.Time) (bool, error) {
	return m.blocked, m.blockedErr
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
func (m *mockEscalationRepo) SetFeedback(_ *gorm.DB, id, feedback, _ string, _ time.Time) error {
	if m.feedbackErr != nil {
		return m.feedbackErr
	}
	m.feedbackSet = append(m.feedbackSet, struct{ id, feedback string }{id, feedback})
	return nil
}
func (m *mockEscalationRepo) CountTodayByNakes(_ *gorm.DB, _ string, _ time.Time) (int64, error) {
	return m.todayCount, m.todayCountErr
}

type mockRiskReader struct {
	status string
	found  bool
	err    error
}

func (m *mockRiskReader) FindLatestStatus(_ *gorm.DB, _, _ string) (string, bool, error) {
	return m.status, m.found, m.err
}

type mockHealthLogReader struct {
	hit bool
	err error
}

func (m *mockHealthLogReader) HasExtremeReadingToday(_ *gorm.DB, _ string, _, _, _, _ float64) (bool, error) {
	return m.hit, m.err
}

type mockNotifier struct {
	nakesCalls     []string
	patientCalls   []string
	companionCalls []string
	nakesErr       error
}

func (m *mockNotifier) SendEscalationToNakes(_ context.Context, phone, _, _, _ string) error {
	m.nakesCalls = append(m.nakesCalls, phone)
	return m.nakesErr
}
func (m *mockNotifier) SendEscalationToPatient(_ context.Context, phone, _ string) error {
	m.patientCalls = append(m.patientCalls, phone)
	return nil
}
func (m *mockNotifier) SendEscalationToCompanion(_ context.Context, phone, _, _ string) error {
	m.companionCalls = append(m.companionCalls, phone)
	return nil
}

type mockNotifRepo struct {
	created  int
	statuses []string
}

func (m *mockNotifRepo) Create(_ *gorm.DB, _ *entity.Notification) error {
	m.created++
	return nil
}
func (m *mockNotifRepo) MarkStatus(_ *gorm.DB, _, status string, _ *string) error {
	m.statuses = append(m.statuses, status)
	return nil
}

type mockInboxRepo struct {
	created int
}

func (m *mockInboxRepo) Create(_ *gorm.DB, _ *entity.PatientNotification) error {
	m.created++
	return nil
}

type mockEscNakesRepo struct {
	nakes *entity.Nakes
	err   error
}

func (m *mockEscNakesRepo) FindByID(_ *gorm.DB, _ string) (*entity.Nakes, error) {
	return m.nakes, m.err
}

type mockPushNotifier struct {
	calls []struct {
		patientID, title, body string
		data                   map[string]string
	}
}

func (m *mockPushNotifier) Notify(_ context.Context, patientID, title, body string, data map[string]string) {
	m.calls = append(m.calls, struct {
		patientID, title, body string
		data                   map[string]string
	}{patientID, title, body, data})
}

func newEscalationUCFull(repo *mockEscalationRepo, risk *mockRiskReader, wa *mockNotifier, nr *mockNotifRepo, inbox *mockInboxRepo, nakes *mockEscNakesRepo, budget int) *usecase.EscalationUseCase {
	return &usecase.EscalationUseCase{
		DB:          nil,
		Repo:        repo,
		RiskRepo:    risk,
		NakesRepo:   nakes,
		WA:          wa,
		NotifRepo:   nr,
		InboxRepo:   inbox,
		AlertBudget: budget,
		Log:         zap.NewNop(),
	}
}

func patientWithContacts() *entity.Patient {
	return &entity.Patient{
		ID: "p1", FaskesID: "f1", AssignedNakesID: "n1",
		FullName: "Budi", PhoneNumber: "628111",
		CompanionName: "Ibu Budi", CompanionPhone: "628222",
	}
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

func TestEvaluateAcute_SkipsWhenBlockedByDedupOrCooldown(t *testing.T) {
	repo := &mockEscalationRepo{blocked: true}
	risk := &mockRiskReader{status: entity.RiskStatusWaswas, found: true}
	uc := newEscalationUC(repo, risk)

	_ = uc.EvaluateAcute(context.Background(), samplePatient(), bahayaScore())
	if len(repo.created) != 0 {
		t.Errorf("expected skip when active/recent escalation exists, got %d created", len(repo.created))
	}
}

func TestEvaluateAcute_FiresOnExtremeReadingWithoutTransition(t *testing.T) {
	repo := &mockEscalationRepo{}
	// status waswas (bukan bahaya) -> tidak ada transisi; pemicu murni dari pembacaan ekstrem.
	risk := &mockRiskReader{status: entity.RiskStatusWaswas, found: true}
	uc := newEscalationUC(repo, risk)
	uc.HealthLogRepo = &mockHealthLogReader{hit: true}

	score := &entity.RiskScore{ID: "rs1", PatientID: "p1", Status: entity.RiskStatusWaswas}
	if err := uc.EvaluateAcute(context.Background(), samplePatient(), score); err != nil {
		t.Fatalf("EvaluateAcute error: %v", err)
	}
	if len(repo.created) != 1 {
		t.Errorf("expected escalation from extreme reading, got %d", len(repo.created))
	}
}

func TestEvaluateAcute_NoTriggerWhenWaswasAndNoExtreme(t *testing.T) {
	repo := &mockEscalationRepo{}
	uc := newEscalationUC(repo, &mockRiskReader{})
	uc.HealthLogRepo = &mockHealthLogReader{hit: false}

	score := &entity.RiskScore{ID: "rs1", PatientID: "p1", Status: entity.RiskStatusWaswas}
	_ = uc.EvaluateAcute(context.Background(), samplePatient(), score)
	if len(repo.created) != 0 {
		t.Errorf("expected no escalation (waswas, no extreme), got %d", len(repo.created))
	}
}

// ── Fan-out tests ───────────────────────────────────────────────────────────

func TestEvaluateAcute_FansOutToAllChannels(t *testing.T) {
	repo := &mockEscalationRepo{}
	risk := &mockRiskReader{status: entity.RiskStatusWaswas, found: true}
	wa := &mockNotifier{}
	nr := &mockNotifRepo{}
	inbox := &mockInboxRepo{}
	nakes := &mockEscNakesRepo{nakes: &entity.Nakes{ID: "n1", FullName: "dr. Sehat", PhoneNumber: "628999"}}
	push := &mockPushNotifier{}
	uc := newEscalationUCFull(repo, risk, wa, nr, inbox, nakes, 0)
	uc.Push = push

	if err := uc.EvaluateAcute(context.Background(), patientWithContacts(), bahayaScore()); err != nil {
		t.Fatalf("EvaluateAcute error: %v", err)
	}
	if len(repo.created) != 1 {
		t.Fatalf("expected 1 escalation, got %d", len(repo.created))
	}
	if inbox.created != 1 {
		t.Errorf("expected 1 inbox row, got %d", inbox.created)
	}
	if len(wa.nakesCalls) != 1 || len(wa.patientCalls) != 1 || len(wa.companionCalls) != 1 {
		t.Errorf("expected 1 WA call each (nakes/patient/companion), got %d/%d/%d",
			len(wa.nakesCalls), len(wa.patientCalls), len(wa.companionCalls))
	}
	if nr.created != 3 {
		t.Errorf("expected 3 notification audit rows, got %d", nr.created)
	}
	if len(push.calls) != 1 {
		t.Fatalf("expected push notify called once for acute escalation, got %d", len(push.calls))
	}
	if push.calls[0].patientID != "p1" {
		t.Fatalf("expected push notify for patient p1, got %s", push.calls[0].patientID)
	}
	if push.calls[0].data["type"] != "escalation" {
		t.Fatalf("expected data type=escalation, got %v", push.calls[0].data)
	}
}

func TestEvaluateAcute_BudgetSkipsWAButKeepsInbox(t *testing.T) {
	repo := &mockEscalationRepo{todayCount: 25} // already over budget
	risk := &mockRiskReader{status: entity.RiskStatusWaswas, found: true}
	wa := &mockNotifier{}
	nr := &mockNotifRepo{}
	inbox := &mockInboxRepo{}
	nakes := &mockEscNakesRepo{nakes: &entity.Nakes{ID: "n1", PhoneNumber: "628999"}}
	uc := newEscalationUCFull(repo, risk, wa, nr, inbox, nakes, 20)

	_ = uc.EvaluateAcute(context.Background(), patientWithContacts(), bahayaScore())
	if len(repo.created) != 1 || inbox.created != 1 {
		t.Errorf("escalation + inbox should still be created; got esc=%d inbox=%d", len(repo.created), inbox.created)
	}
	if len(wa.nakesCalls)+len(wa.patientCalls)+len(wa.companionCalls) != 0 {
		t.Errorf("expected no WA sends when over budget, got %d/%d/%d",
			len(wa.nakesCalls), len(wa.patientCalls), len(wa.companionCalls))
	}
}

func TestEvaluateAcute_SkipsCompanionWAWhenNoPhone(t *testing.T) {
	repo := &mockEscalationRepo{}
	risk := &mockRiskReader{found: false}
	wa := &mockNotifier{}
	nr := &mockNotifRepo{}
	inbox := &mockInboxRepo{}
	nakes := &mockEscNakesRepo{nakes: &entity.Nakes{ID: "n1", PhoneNumber: "628999"}}
	uc := newEscalationUCFull(repo, risk, wa, nr, inbox, nakes, 0)

	p := patientWithContacts()
	p.CompanionPhone = ""
	_ = uc.EvaluateAcute(context.Background(), p, bahayaScore())
	if len(wa.companionCalls) != 0 {
		t.Errorf("expected no companion WA when phone empty, got %v", wa.companionCalls)
	}
	if len(wa.patientCalls) != 1 {
		t.Errorf("expected patient WA still sent, got %d", len(wa.patientCalls))
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

// ── Feedback tests ──────────────────────────────────────────────────────────

func TestSetFeedback_Accurate(t *testing.T) {
	repo := &mockEscalationRepo{byID: &entity.Escalation{ID: "e1", FaskesID: "f1", Status: entity.EscalationStatusActed}}
	uc := newEscalationUC(repo, &mockRiskReader{})

	if err := uc.SetFeedback(context.Background(), "e1", "f1", "n1", entity.EscalationFeedbackAccurate); err != nil {
		t.Fatalf("SetFeedback returned error: %v", err)
	}
	if len(repo.feedbackSet) != 1 || repo.feedbackSet[0].feedback != entity.EscalationFeedbackAccurate {
		t.Errorf("expected feedback accurate set, got %v", repo.feedbackSet)
	}
}

func TestSetFeedback_InvalidValueRejected(t *testing.T) {
	repo := &mockEscalationRepo{byID: &entity.Escalation{ID: "e1", FaskesID: "f1"}}
	uc := newEscalationUC(repo, &mockRiskReader{})

	err := uc.SetFeedback(context.Background(), "e1", "f1", "n1", "maybe")
	if err != usecase.ErrInvalidFeedback {
		t.Errorf("expected ErrInvalidFeedback, got %v", err)
	}
	if len(repo.feedbackSet) != 0 {
		t.Errorf("expected no feedback set on invalid value, got %v", repo.feedbackSet)
	}
}

func TestSetFeedback_ForeignFaskesIsNotFound(t *testing.T) {
	repo := &mockEscalationRepo{byID: &entity.Escalation{ID: "e1", FaskesID: "OTHER"}}
	uc := newEscalationUC(repo, &mockRiskReader{})

	err := uc.SetFeedback(context.Background(), "e1", "f1", "n1", entity.EscalationFeedbackAccurate)
	if err != usecase.ErrEscalationNotFound {
		t.Errorf("expected ErrEscalationNotFound for foreign faskes, got %v", err)
	}
}
