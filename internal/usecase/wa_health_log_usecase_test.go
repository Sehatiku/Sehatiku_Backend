package usecase

import (
	"context"
	"errors"
	"testing"
	"time"

	"sehatiku-backend/internal/entity"
	"sehatiku-backend/internal/helper"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

// ── Mocks ─────────────────────────────────────────────────────────────────────

type mockWAPatientRepo struct {
	byPhone          *entity.Patient
	byPhoneErr       error
	byCompanionPhone *entity.Patient
	byCompanionErr   error
}

func (m *mockWAPatientRepo) FindByPhone(_ *gorm.DB, _ string) (*entity.Patient, error) {
	return m.byPhone, m.byPhoneErr
}
func (m *mockWAPatientRepo) FindByCompanionPhone(_ *gorm.DB, _ string) (*entity.Patient, error) {
	return m.byCompanionPhone, m.byCompanionErr
}

type mockWALogRepo struct {
	created []*entity.HealthLog
	err     error
}

func (m *mockWALogRepo) Create(_ *gorm.DB, log *entity.HealthLog) error {
	if m.err != nil {
		return m.err
	}
	m.created = append(m.created, log)
	return nil
}

type mockWAReplySender struct {
	confirmationCalls  int
	batchCalls         int
	batchItems         []string
	templateCalls      int
	parseErrorCalls    int
	notRegisteredCalls int
}

func (m *mockWAReplySender) SendHealthLogConfirmation(_ context.Context, _, _, _, _ string) error {
	m.confirmationCalls++
	return nil
}
func (m *mockWAReplySender) SendHealthLogBatchConfirmation(_ context.Context, _, _ string, items []string) error {
	m.batchCalls++
	m.batchItems = items
	return nil
}
func (m *mockWAReplySender) SendLogTemplate(_ context.Context, _, _ string) error {
	m.templateCalls++
	return nil
}
func (m *mockWAReplySender) SendHealthLogParseError(_ context.Context, _ string) error {
	m.parseErrorCalls++
	return nil
}
func (m *mockWAReplySender) SendHealthLogNotRegistered(_ context.Context, _ string) error {
	m.notRegisteredCalls++
	return nil
}

var testPatient = &entity.Patient{
	ID:          "patient-uuid-1",
	FullName:    "Budi Santoso",
	PhoneNumber: "628111222333",
	Status:      "active",
}

func newWAHealthLogUC(pr *mockWAPatientRepo, lr *mockWALogRepo, wa *mockWAReplySender) *WAHealthLogUseCase {
	return &WAHealthLogUseCase{
		DB:          nil,
		PatientRepo: pr,
		LogRepo:     lr,
		Extractor:   nil,
		WhatsApp:    wa,
		Log:         zap.NewNop(),
	}
}

// ── Tests ─────────────────────────────────────────────────────────────────────

func TestWAHealthLog_GlucoseFromPatientPhone(t *testing.T) {
	pr := &mockWAPatientRepo{byPhone: testPatient}
	lr := &mockWALogRepo{}
	wa := &mockWAReplySender{}
	uc := newWAHealthLogUC(pr, lr, wa)

	err := uc.HandleInbound(context.Background(), "628111222333", "gula 180")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(lr.created) != 1 {
		t.Fatalf("expected 1 log created, got %d", len(lr.created))
	}
	log := lr.created[0]
	if log.MetricType != "glucose" {
		t.Errorf("metric_type = %s; want glucose", log.MetricType)
	}
	if log.ValueNumeric == nil || *log.ValueNumeric != 180 {
		t.Errorf("value_numeric = %v; want 180", log.ValueNumeric)
	}
	if log.Source != entity.LogSourceWhatsApp {
		t.Errorf("source = %s; want whatsapp", log.Source)
	}
	if log.LoggedBy != entity.LoggedByPatient {
		t.Errorf("logged_by = %s; want patient", log.LoggedBy)
	}
	if wa.confirmationCalls != 1 {
		t.Errorf("confirmationCalls = %d; want 1", wa.confirmationCalls)
	}
}

func TestWAHealthLog_BPFromCompanionPhone(t *testing.T) {
	pr := &mockWAPatientRepo{
		byPhoneErr:       gorm.ErrRecordNotFound,
		byCompanionPhone: testPatient,
	}
	lr := &mockWALogRepo{}
	wa := &mockWAReplySender{}
	uc := newWAHealthLogUC(pr, lr, wa)

	err := uc.HandleInbound(context.Background(), "628999000111", "tensi 130/85")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(lr.created) != 1 {
		t.Fatalf("expected 1 log created, got %d", len(lr.created))
	}
	log := lr.created[0]
	if log.MetricType != "bp" {
		t.Errorf("metric_type = %s; want bp", log.MetricType)
	}
	if log.LoggedBy != entity.LoggedByCompanion {
		t.Errorf("logged_by = %s; want companion", log.LoggedBy)
	}
	if log.ValueJSONB == nil {
		t.Error("value_jsonb nil; want BP JSON")
	}
	if wa.confirmationCalls != 1 {
		t.Errorf("confirmationCalls = %d; want 1", wa.confirmationCalls)
	}
}

func TestWAHealthLog_NotRegistered(t *testing.T) {
	pr := &mockWAPatientRepo{
		byPhoneErr:     gorm.ErrRecordNotFound,
		byCompanionErr: gorm.ErrRecordNotFound,
	}
	lr := &mockWALogRepo{}
	wa := &mockWAReplySender{}
	uc := newWAHealthLogUC(pr, lr, wa)

	err := uc.HandleInbound(context.Background(), "6200000", "gula 180")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(lr.created) != 0 {
		t.Errorf("expected no log created, got %d", len(lr.created))
	}
	if wa.notRegisteredCalls != 1 {
		t.Errorf("notRegisteredCalls = %d; want 1", wa.notRegisteredCalls)
	}
}

func TestWAHealthLog_UnknownMessage(t *testing.T) {
	pr := &mockWAPatientRepo{byPhone: testPatient}
	lr := &mockWALogRepo{}
	wa := &mockWAReplySender{}
	uc := newWAHealthLogUC(pr, lr, wa)

	err := uc.HandleInbound(context.Background(), "628111222333", "halo apa kabar")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(lr.created) != 0 {
		t.Errorf("expected no log created, got %d", len(lr.created))
	}
	if wa.parseErrorCalls != 1 {
		t.Errorf("parseErrorCalls = %d; want 1", wa.parseErrorCalls)
	}
}

func TestWAHealthLog_DBError_Propagated(t *testing.T) {
	pr := &mockWAPatientRepo{byPhoneErr: errors.New("connection refused")}
	uc := newWAHealthLogUC(pr, &mockWALogRepo{}, &mockWAReplySender{})

	err := uc.HandleInbound(context.Background(), "628111", "gula 180")
	if err == nil {
		t.Error("expected error when DB fails, got nil")
	}
}

func TestWAHealthLog_InsertError_Propagated(t *testing.T) {
	pr := &mockWAPatientRepo{byPhone: testPatient}
	lr := &mockWALogRepo{err: errors.New("unique constraint violated")}
	wa := &mockWAReplySender{}
	uc := newWAHealthLogUC(pr, lr, wa)

	err := uc.HandleInbound(context.Background(), "628111", "gula 180")
	if err == nil {
		t.Error("expected error when insert fails, got nil")
	}
}

func TestWAHealthLog_MedAdherence_Yes(t *testing.T) {
	pr := &mockWAPatientRepo{byPhone: testPatient}
	lr := &mockWALogRepo{}
	uc := newWAHealthLogUC(pr, lr, &mockWAReplySender{})

	if err := uc.HandleInbound(context.Background(), "628111", "minum obat"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(lr.created) != 1 || lr.created[0].MetricType != "med_adherence" {
		t.Errorf("expected med_adherence log, got %+v", lr.created)
	}
	if *lr.created[0].ValueNumeric != 100 {
		t.Errorf("expected 100%% adherence, got %v", lr.created[0].ValueNumeric)
	}
}

func TestWAHealthLog_MeasuredAt_IsNow(t *testing.T) {
	pr := &mockWAPatientRepo{byPhone: testPatient}
	lr := &mockWALogRepo{}
	before := time.Now()
	uc := newWAHealthLogUC(pr, lr, &mockWAReplySender{})

	_ = uc.HandleInbound(context.Background(), "628111", "tidur 7 jam")

	after := time.Now()
	if len(lr.created) != 1 {
		t.Fatal("no log created")
	}
	measuredAt := lr.created[0].MeasuredAt
	// MeasuredAt WA diset ke time.Now() saat diproses — wajar ada jitter kecil
	// tapi tidak boleh jauh di luar window before/after.
	if measuredAt.Before(before.Add(-time.Second)) || measuredAt.After(after.Add(time.Second)) {
		t.Errorf("measuredAt %v di luar window [%v, %v]", measuredAt, before, after)
	}
}

func TestWAHealthLog_TemplateRequest(t *testing.T) {
	pr := &mockWAPatientRepo{byPhone: testPatient}
	lr := &mockWALogRepo{}
	wa := &mockWAReplySender{}
	uc := newWAHealthLogUC(pr, lr, wa)

	if err := uc.HandleInbound(context.Background(), "628111222333", "saya ingin tulis log harian"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if wa.templateCalls != 1 {
		t.Errorf("templateCalls = %d; want 1", wa.templateCalls)
	}
	if len(lr.created) != 0 {
		t.Errorf("expected no log created, got %d", len(lr.created))
	}
	if wa.confirmationCalls != 0 || wa.parseErrorCalls != 0 {
		t.Errorf("template request should not confirm/parse-error")
	}
}

func TestWAHealthLog_MultiMetricForm(t *testing.T) {
	pr := &mockWAPatientRepo{byPhone: testPatient}
	lr := &mockWALogRepo{}
	wa := &mockWAReplySender{}
	uc := newWAHealthLogUC(pr, lr, wa)

	msg := "Gula: 180\nTensi: 120/80\nMakan: nasi goreng\nStres: 4\nObat: tidak\nBerat: 65"
	if err := uc.HandleInbound(context.Background(), "628111222333", msg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(lr.created) != 6 {
		t.Fatalf("expected 6 logs, got %d", len(lr.created))
	}
	// Fitur AI kritis harus ikut tercatat: food (carbs/sodium) & stress.
	got := map[string]bool{}
	for _, l := range lr.created {
		got[l.MetricType] = true
	}
	for _, want := range []string{"glucose", "bp", "food", "stress", "med_adherence", "weight"} {
		if !got[want] {
			t.Errorf("metric %s tidak tercatat dari form", want)
		}
	}
	if wa.batchCalls != 1 {
		t.Errorf("batchCalls = %d; want 1", wa.batchCalls)
	}
	if wa.confirmationCalls != 0 {
		t.Errorf("single confirmation should not fire for a form; got %d", wa.confirmationCalls)
	}
	// Cari metrik obat — nilai "tidak" harus tersimpan sebagai 0, bukan 100.
	var found bool
	for _, l := range lr.created {
		if l.MetricType == "med_adherence" {
			found = true
			if l.ValueNumeric == nil || *l.ValueNumeric != 0 {
				t.Errorf("obat 'tidak' = %v; want 0", l.ValueNumeric)
			}
		}
		if l.Source != entity.LogSourceWhatsApp {
			t.Errorf("source = %s; want whatsapp", l.Source)
		}
	}
	if !found {
		t.Error("med_adherence log not found in form")
	}
}

// Baris terisi sebagian: kolom kosong & placeholder underscore harus dilewati,
// hanya baris berisi yang tercatat.
func TestWAHealthLog_PartialForm(t *testing.T) {
	pr := &mockWAPatientRepo{byPhone: testPatient}
	lr := &mockWALogRepo{}
	wa := &mockWAReplySender{}
	uc := newWAHealthLogUC(pr, lr, wa)

	msg := "Gula: 200\nTensi: \nObat: ___\nBerat: 70"
	if err := uc.HandleInbound(context.Background(), "628111222333", msg); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(lr.created) != 2 {
		t.Fatalf("expected 2 logs (gula, berat), got %d", len(lr.created))
	}
}

func TestMetricDisplay(t *testing.T) {
	v := 180.0
	label, val := metricDisplay(helper.ParsedMetric{MetricType: "glucose", ValueNumeric: &v})
	if label == "" || val == "" {
		t.Errorf("metricDisplay returned empty strings for glucose")
	}

	sys, dia := 130, 85
	label, val = metricDisplay(helper.ParsedMetric{MetricType: "bp", BPSystolic: &sys, BPDiastolic: &dia})
	if label == "" || val == "" {
		t.Errorf("metricDisplay returned empty strings for bp")
	}
}
