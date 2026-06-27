package whatsapp

import (
	"context"
	"fmt"
	"time"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"
)

type WhatsAppGateway struct {
	Client *whatsmeow.Client
	Log    *zap.Logger
}

func New(client *whatsmeow.Client, log *zap.Logger) *WhatsAppGateway {
	return &WhatsAppGateway{Client: client, Log: log}
}

// SendLoginNotification mengirim notifikasi teks ke nomor WA saat login berhasil.
// Dipanggil secara fire-and-forget dari goroutine — error hanya di-log, tidak dipropagasi.
func (g *WhatsAppGateway) SendLoginNotification(ctx context.Context, toPhone, recipientName string) error {
	if g.Client == nil || !g.Client.IsConnected() {
		return fmt.Errorf("whatsapp client not connected")
	}

	jid := types.NewJID(toPhone, types.DefaultUserServer)

	text := fmt.Sprintf(
		"[Sehatiku] Halo %s, login ke akun Sehatiku berhasil pada %s. Jika bukan Anda yang login, segera hubungi admin faskes Anda.",
		recipientName,
		time.Now().Format("02 Jan 2006 15:04 WIB"),
	)

	_, err := g.Client.SendMessage(ctx, jid, &waE2E.Message{
		Conversation: proto.String(text),
	})
	if err != nil {
		return fmt.Errorf("sending wa login notification to %s: %w", toPhone, err)
	}

	g.Log.Info("wa login notification sent", zap.String("to", toPhone))
	return nil
}
