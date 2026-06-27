package whatsapp

import (
	"context"
	"fmt"
	"time"

	"sehatiku-backend/internal/helper"

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
	text := fmt.Sprintf(
		"🔐 *Sehatiku — Notifikasi Login*\n\nHalo %s 👋\n\nKami mendeteksi login ke akun Sehatiku Anda pada *%s*.\n\nJika ini Anda, tidak ada tindakan yang diperlukan ✅\nNamun jika Anda *tidak* merasa melakukan login ini, segera hubungi admin faskes Anda untuk mengamankan akun 🚨\n\nTerima kasih telah menjaga kesehatan bersama Sehatiku 💙",
		recipientName,
		time.Now().Format("02 Jan 2006 15:04 WIB"),
	)

	if err := g.sendText(ctx, toPhone, text); err != nil {
		return fmt.Errorf("sending wa login notification to %s: %w", toPhone, err)
	}

	g.Log.Info("wa login notification sent", zap.String("to", toPhone))
	return nil
}

// SendRegistrationCredentials mengirim username & password akun ke nomor WA pemilik
// akun (nakes atau pasien) yang baru didaftarkan. Akun dibuatkan oleh pihak lain
// (admin faskes / nakes), jadi WA adalah kanal untuk menyampaikan kredensial login.
// Dipanggil fire-and-forget — error hanya di-log, tidak dipropagasi.
func (g *WhatsAppGateway) SendRegistrationCredentials(ctx context.Context, toPhone, recipientName, username, password string) error {
	text := fmt.Sprintf(
		"🎉 *Selamat Datang di Sehatiku!*\n\nHalo %s 👋\n\nAkun Sehatiku Anda telah berhasil dibuat. Berikut kredensial login Anda:\n\n👤 Username: *%s*\n🔑 Password: *%s*\n\n⚠️ *Penting demi keamanan akun Anda:*\n• Simpan pesan ini dengan baik 🗂️\n• Jangan pernah membagikan kredensial ini kepada siapa pun 🤫\n• Segera ganti password setelah login pertama 🔄\n\nSelamat memulai perjalanan sehat Anda bersama kami 💙",
		recipientName, username, password,
	)

	if err := g.sendText(ctx, toPhone, text); err != nil {
		return fmt.Errorf("sending wa registration credentials to %s: %w", toPhone, err)
	}

	g.Log.Info("wa registration credentials sent", zap.String("to", toPhone))
	return nil
}

// SendCompanionRegistrationCredentials memberi tahu pendamping bahwa pasien telah
// didaftarkan, beserta kredensial login pasien supaya pendamping bisa membantu pasien
// lansia mengakses akunnya. Dipanggil fire-and-forget — error hanya di-log.
func (g *WhatsAppGateway) SendCompanionRegistrationCredentials(ctx context.Context, toPhone, companionName, patientName, username, password string) error {
	text := fmt.Sprintf(
		"🤝 *Sehatiku — Informasi Akun Pasien*\n\nHalo %s 👋\n\n%s telah berhasil didaftarkan di Sehatiku. Sebagai pendamping, kami mohon bantuan Anda untuk mendampingi *%s* login dan mencatat data kesehatannya setiap hari 📋\n\nBerikut kredensial login pasien:\n\n👤 Username: *%s*\n🔑 Password: *%s*\n\n⚠️ Mohon jaga kerahasiaan kredensial ini dan jangan membagikannya kepada pihak lain 🔒\n\nTerima kasih atas kepedulian Anda 💙",
		companionName, patientName, patientName, username, password,
	)

	if err := g.sendText(ctx, toPhone, text); err != nil {
		return fmt.Errorf("sending wa companion registration notification to %s: %w", toPhone, err)
	}

	g.Log.Info("wa companion registration notification sent", zap.String("to", toPhone))
	return nil
}

// sendText adalah helper internal untuk mengirim pesan teks biasa ke satu nomor WA.
func (g *WhatsAppGateway) sendText(ctx context.Context, toPhone, text string) error {
	if g.Client == nil || !g.Client.IsConnected() {
		return fmt.Errorf("whatsapp client not connected")
	}

	phone := helper.NormalizePhoneID(toPhone)
	if phone == "" {
		return fmt.Errorf("invalid phone number %q", toPhone)
	}

	jid := types.NewJID(phone, types.DefaultUserServer)

	_, err := g.Client.SendMessage(ctx, jid, &waE2E.Message{
		Conversation: proto.String(text),
	})
	return err
}
