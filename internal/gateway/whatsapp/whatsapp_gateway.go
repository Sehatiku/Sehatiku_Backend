package whatsapp

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"sehatiku-backend/internal/helper"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/proto/waE2E"
	"go.mau.fi/whatsmeow/types"
	"go.uber.org/zap"
	"google.golang.org/protobuf/proto"
)

// ErrColdContactBlocked menandai kegagalan kirim karena WhatsApp memblokir pengiriman ke
// kontak dingin — server membalas error 463 (NackCallerReachoutTimelocked). Ini BUKAN bug
// kode: WhatsApp membatasi akun yang mengirim ke nomor yang belum pernah menghubungi bot
// lebih dulu. Solusinya adalah alur warm-up (penerima menghubungi bot dahulu lewat link
// wa.me), bukan retry. Sentinel ini memisahkan kegagalan "wajar & diketahui" ini dari
// kegagalan tak terduga supaya log & status tidak menyesatkan.
var ErrColdContactBlocked = errors.New("whatsapp memblokir pengiriman ke kontak baru (reachout time-lock, error 463)")

// isReachoutTimelocked mengenali error 463 dari ack server whatsmeow
// (fmt.Errorf("%w %d", ErrServerReturnedError, 463)).
func isReachoutTimelocked(err error) bool {
	return errors.Is(err, whatsmeow.ErrServerReturnedError) && strings.Contains(err.Error(), "463")
}

// waConnectWaitTimeout adalah batas waktu menunggu socket WA pulih sebelum sebuah
// pengiriman pesan menyerah. Dengan EnableAutoReconnect aktif, koneksi yang putus
// sementara (mis. server WhatsApp menutup websocket lalu client reconnect) biasanya
// pulih dalam hitungan detik. Tanpa menunggu, pesan fire-and-forget akan langsung
// hilang setiap kali kebetulan dikirim di celah reconnect tersebut.
const waConnectWaitTimeout = 30 * time.Second

type WhatsAppGateway struct {
	Client *whatsmeow.Client
	Log    *zap.Logger
}

func New(client *whatsmeow.Client, log *zap.Logger) *WhatsAppGateway {
	return &WhatsAppGateway{Client: client, Log: log}
}

// BotPhone mengembalikan nomor WA bot (format internasional telanjang, mis. "62812...")
// untuk membangun link wa.me first-contact. Mengembalikan "" bila device belum dipasangkan
// (Store.ID nil) — pemanggil memakai ini untuk menandai link tidak tersedia.
func (g *WhatsAppGateway) BotPhone() string {
	if g.Client == nil || g.Client.Store == nil || g.Client.Store.ID == nil {
		return ""
	}
	return g.Client.Store.ID.User
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
	if g.Client == nil {
		return fmt.Errorf("whatsapp client not initialised")
	}

	// Bedakan "belum pernah dipasangkan" dari "putus sementara": kalau tidak ada sesi
	// di store (Store.ID == nil), tidak ada gunanya menunggu — device harus scan QR
	// dulu via cmd/wa-setup. Pesan error ini sengaja eksplisit supaya log langsung
	// menunjukkan akar masalahnya, bukan sekadar "not connected".
	if g.Client.Store.ID == nil {
		return fmt.Errorf("whatsapp client not paired (jalankan cmd/wa-setup untuk scan QR)")
	}

	// Device terpasang tapi socket sedang tidak terhubung — kemungkinan besar sedang
	// reconnect. Tunggu sampai koneksi pulih (atau timeout) sebelum menyerah, supaya
	// kredensial tidak hilang hanya karena kebetulan dikirim di celah reconnect.
	// Pemanggil sinkron (mis. registrasi pasien) menetapkan deadline lewat ctx supaya
	// request tidak menggantung 30 detik saat WA putus; pemanggil fire-and-forget tanpa
	// deadline memakai waConnectWaitTimeout penuh.
	if !g.Client.IsConnected() {
		waitTimeout := waConnectWaitTimeout
		if dl, ok := ctx.Deadline(); ok {
			if remaining := time.Until(dl); remaining < waitTimeout {
				waitTimeout = remaining
			}
		}
		if waitTimeout <= 0 || !g.Client.WaitForConnection(waitTimeout) {
			return fmt.Errorf("whatsapp client not connected (timeout menunggu reconnect)")
		}
	}

	phone := helper.NormalizePhoneID(toPhone)
	if phone == "" {
		return fmt.Errorf("invalid phone number %q", toPhone)
	}

	jid := types.NewJID(phone, types.DefaultUserServer)

	_, err := g.Client.SendMessage(ctx, jid, &waE2E.Message{
		Conversation: proto.String(text),
	})
	if err != nil && isReachoutTimelocked(err) {
		return fmt.Errorf("%w: %v", ErrColdContactBlocked, err)
	}
	return err
}
