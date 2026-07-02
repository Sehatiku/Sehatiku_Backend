package push

import (
	"context"
	"fmt"

	"firebase.google.com/go/v4/messaging"
	"go.uber.org/zap"
)

// PushGateway membungkus Firebase Cloud Messaging Admin SDK. Client nil-able: bila
// kredensial Firebase belum dikonfigurasi (lihat internal/config/push.go), Client tetap nil
// dan SendMulticast mengembalikan error yang di-log oleh caller — server tetap start normal,
// sama falsafah graceful-degradation dengan WhatsAppGateway saat device belum dipasangkan.
type PushGateway struct {
	Client *messaging.Client
	Log    *zap.Logger
}

// SendMulticast mengirim satu notifikasi ke banyak token FCM sekaligus. Mengembalikan token
// yang FCM tandai invalid/unregistered (untuk dinonaktifkan pemanggil) terpisah dari error
// pengiriman keseluruhan (mis. gateway belum dikonfigurasi, FCM down).
func (g *PushGateway) SendMulticast(ctx context.Context, tokens []string, title, body string, data map[string]string) ([]string, error) {
	if g.Client == nil {
		return nil, fmt.Errorf("push gateway not initialised (FIREBASE_CREDENTIALS_FILE kosong/gagal)")
	}
	if len(tokens) == 0 {
		return nil, nil
	}

	msg := &messaging.MulticastMessage{
		Tokens: tokens,
		Notification: &messaging.Notification{
			Title: title,
			Body:  body,
		},
		Data: data,
	}

	resp, err := g.Client.SendEachForMulticast(ctx, msg)
	if err != nil {
		return nil, fmt.Errorf("sending fcm multicast: %w", err)
	}

	var invalidTokens []string
	for i, r := range resp.Responses {
		if r.Success {
			continue
		}
		if isUnregisteredToken(r.Error) {
			invalidTokens = append(invalidTokens, tokens[i])
		}
		g.Log.Warn("fcm send failed for token", zap.Int("index", i), zap.Error(r.Error))
	}
	return invalidTokens, nil
}

// isUnregisteredToken mengenali token yang FCM tandai sudah tidak terdaftar/invalid, via
// helper resmi SDK (messaging.IsUnregistered) yang membaca kode error terstruktur SDK —
// bukan string-matching pada err.Error() (teks pesan FCM tidak membawa kode error tersebut).
func isUnregisteredToken(err error) bool {
	return err != nil && messaging.IsUnregistered(err)
}
