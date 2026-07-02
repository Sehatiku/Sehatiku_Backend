// internal/usecase/push_usecase_test.go
package usecase_test

import (
	"context"
	"errors"
	"testing"

	"sehatiku-backend/internal/usecase"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

type mockPushTokenRepo struct {
	tokens        []string
	findErr       error
	deactivated   []string
	deactivateErr error
}

func (m *mockPushTokenRepo) FindActiveByPatient(_ *gorm.DB, _ string) ([]string, error) {
	return m.tokens, m.findErr
}
func (m *mockPushTokenRepo) DeactivateTokens(_ *gorm.DB, tokens []string) error {
	if m.deactivateErr != nil {
		return m.deactivateErr
	}
	m.deactivated = append(m.deactivated, tokens...)
	return nil
}

type mockPushSender struct {
	calls         int
	gotTokens     []string
	gotTitle      string
	gotBody       string
	gotData       map[string]string
	invalidTokens []string
	sendErr       error
}

func (m *mockPushSender) SendMulticast(_ context.Context, tokens []string, title, body string, data map[string]string) ([]string, error) {
	m.calls++
	m.gotTokens = tokens
	m.gotTitle = title
	m.gotBody = body
	m.gotData = data
	return m.invalidTokens, m.sendErr
}

func TestPushUseCase_Notify_NoActiveTokens_GatewayNotCalled(t *testing.T) {
	tokenRepo := &mockPushTokenRepo{tokens: nil}
	sender := &mockPushSender{}
	u := &usecase.PushUseCase{TokenRepo: tokenRepo, Gateway: sender, Log: zap.NewNop()}

	u.Notify(context.Background(), "patient-1", "Judul", "Isi", map[string]string{"type": "escalation"})

	if sender.calls != 0 {
		t.Fatalf("expected gateway not called when no active tokens, got %d calls", sender.calls)
	}
}

func TestPushUseCase_Notify_SendsToActiveTokens(t *testing.T) {
	tokenRepo := &mockPushTokenRepo{tokens: []string{"tok-a", "tok-b"}}
	sender := &mockPushSender{}
	u := &usecase.PushUseCase{TokenRepo: tokenRepo, Gateway: sender, Log: zap.NewNop()}

	u.Notify(context.Background(), "patient-1", "Judul", "Isi", map[string]string{"type": "escalation"})

	if sender.calls != 1 {
		t.Fatalf("expected gateway called once, got %d", sender.calls)
	}
	if len(sender.gotTokens) != 2 || sender.gotTokens[0] != "tok-a" || sender.gotTokens[1] != "tok-b" {
		t.Fatalf("expected tokens [tok-a tok-b], got %v", sender.gotTokens)
	}
	if sender.gotTitle != "Judul" || sender.gotBody != "Isi" {
		t.Fatalf("expected title/body passed through, got %q/%q", sender.gotTitle, sender.gotBody)
	}
	if sender.gotData["type"] != "escalation" {
		t.Fatalf("expected data payload passed through, got %v", sender.gotData)
	}
}

func TestPushUseCase_Notify_DeactivatesInvalidTokens(t *testing.T) {
	tokenRepo := &mockPushTokenRepo{tokens: []string{"tok-a", "tok-bad"}}
	sender := &mockPushSender{invalidTokens: []string{"tok-bad"}}
	u := &usecase.PushUseCase{TokenRepo: tokenRepo, Gateway: sender, Log: zap.NewNop()}

	u.Notify(context.Background(), "patient-1", "Judul", "Isi", nil)

	if len(tokenRepo.deactivated) != 1 || tokenRepo.deactivated[0] != "tok-bad" {
		t.Fatalf("expected tok-bad deactivated, got %v", tokenRepo.deactivated)
	}
}

func TestPushUseCase_Notify_GatewayError_DoesNotPanic(t *testing.T) {
	tokenRepo := &mockPushTokenRepo{tokens: []string{"tok-a"}}
	sender := &mockPushSender{sendErr: errors.New("fcm down")}
	u := &usecase.PushUseCase{TokenRepo: tokenRepo, Gateway: sender, Log: zap.NewNop()}

	u.Notify(context.Background(), "patient-1", "Judul", "Isi", nil)
	// No assertion beyond "did not panic" — best-effort semantics per design spec.
}
