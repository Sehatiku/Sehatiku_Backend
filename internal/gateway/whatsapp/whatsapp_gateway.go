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

// SendConsultationReply notifies the patient that their doctor has replied to their
// consultation. Dipanggil fire-and-forget — error hanya di-log, tidak dipropagasi.
func (g *WhatsAppGateway) SendConsultationReply(ctx context.Context, toPhone, patientName, nakesNote string) error {
	text := fmt.Sprintf(
		"💬 *Sehatiku — Balasan Dokter*\n\nHalo %s 👋\n\nDokter Anda telah membalas keluhan Anda:\n\n_%s_\n\nSilakan cek aplikasi Sehatiku untuk melihat rincian selengkapnya 📱",
		patientName,
		nakesNote,
	)
	if err := g.sendText(ctx, toPhone, text); err != nil {
		return fmt.Errorf("sending wa consultation reply to %s: %w", toPhone, err)
	}
	g.Log.Info("wa consultation reply sent", zap.String("to", toPhone))
	return nil
}

// SendEscalationToNakes memberi tahu nakes bahwa seorang pasiennya butuh perhatian akut.
// Dipanggil fire-and-forget — error hanya di-log/dicatat sebagai notifications.status=failed.
func (g *WhatsAppGateway) SendEscalationToNakes(ctx context.Context, toPhone, nakesName, patientName, riskStatus string) error {
	text := fmt.Sprintf(
		"🚨 *Sehatiku — Eskalasi Pasien*\n\nHalo %s 👋\n\nPasien Anda *%s* terdeteksi berstatus *%s* dan perlu perhatian segera.\n\nMohon buka dashboard Sehatiku untuk meninjau detail dan menindaklanjuti 🩺",
		nakesName, patientName, riskStatus,
	)
	if err := g.sendText(ctx, toPhone, text); err != nil {
		return fmt.Errorf("sending wa escalation to nakes %s: %w", toPhone, err)
	}
	g.Log.Info("wa escalation sent to nakes", zap.String("to", toPhone))
	return nil
}

// SendEscalationToPatient memberi tahu pasien bahwa kondisinya perlu perhatian dan tim
// kesehatannya sudah diberi tahu. Bahasa ramah-lansia, tanpa detail klinis menakutkan.
func (g *WhatsAppGateway) SendEscalationToPatient(ctx context.Context, toPhone, patientName string) error {
	text := fmt.Sprintf(
		"💙 *Sehatiku*\n\nHalo %s 👋\n\nKondisi kesehatan Anda hari ini perlu perhatian. Tim kesehatan Anda sudah kami beri tahu.\n\nMohon segera hubungi atau kunjungi faskes Anda ya 🙏",
		patientName,
	)
	if err := g.sendText(ctx, toPhone, text); err != nil {
		return fmt.Errorf("sending wa escalation to patient %s: %w", toPhone, err)
	}
	g.Log.Info("wa escalation sent to patient", zap.String("to", toPhone))
	return nil
}

// SendEscalationToCompanion meminta pendamping membantu pasien (sering lansia) segera
// menghubungi faskes.
func (g *WhatsAppGateway) SendEscalationToCompanion(ctx context.Context, toPhone, companionName, patientName string) error {
	text := fmt.Sprintf(
		"🤝 *Sehatiku — Mohon Bantuan Anda*\n\nHalo %s 👋\n\nKondisi *%s* hari ini perlu perhatian. Mohon bantu beliau untuk segera menghubungi atau mengunjungi faskes 🙏\n\nTerima kasih atas kepedulian Anda 💙",
		companionName, patientName,
	)
	if err := g.sendText(ctx, toPhone, text); err != nil {
		return fmt.Errorf("sending wa escalation to companion %s: %w", toPhone, err)
	}
	g.Log.Info("wa escalation sent to companion", zap.String("to", toPhone))
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

// SendHealthLogConfirmation mengirim balasan ke pasien/pendamping setelah data harian
// berhasil disimpan. Pesan singkat, ramah-lansia, dengan emoji agar mudah dibaca.
func (g *WhatsAppGateway) SendHealthLogConfirmation(ctx context.Context, toPhone, patientName, metricLabel, valueStr string) error {
	text := fmt.Sprintf(
		"✅ *Sehatiku — Data Berhasil Dicatat*\n\nHalo %s 👋\n\n%s: *%s* sudah kami simpan.\n\nTerima kasih sudah rutin mencatat ya 💙",
		patientName, metricLabel, valueStr,
	)
	if err := g.sendText(ctx, toPhone, text); err != nil {
		return fmt.Errorf("sending health log confirmation to %s: %w", toPhone, err)
	}
	g.Log.Info("wa health log confirmation sent", zap.String("to", toPhone))
	return nil
}

// SendHealthLogBatchConfirmation mengirim konfirmasi untuk beberapa metrik sekaligus
// (hasil pengisian template log harian). `items` sudah berformat "Label: nilai".
func (g *WhatsAppGateway) SendHealthLogBatchConfirmation(ctx context.Context, toPhone, patientName string, items []string) error {
	var b strings.Builder
	fmt.Fprintf(&b, "✅ *Sehatiku — Data Berhasil Dicatat*\n\nHalo %s 👋\n\nKami sudah menyimpan:\n", patientName)
	for _, it := range items {
		b.WriteString("• ")
		b.WriteString(it)
		b.WriteString("\n")
	}
	fmt.Fprintf(&b, "\n🙏 *Terima kasih, %s!* Log harian Anda sudah lengkap kami terima.\nTerus jaga kesehatan ya 💙", patientName)
	if err := g.sendText(ctx, toPhone, b.String()); err != nil {
		return fmt.Errorf("sending batch health log confirmation to %s: %w", toPhone, err)
	}
	g.Log.Info("wa health log batch confirmation sent", zap.String("to", toPhone), zap.Int("count", len(items)))
	return nil
}

// SendLogTemplate mengirim template log harian kosong untuk diisi pasien/pendamping,
// dipicu saat pengirim meminta panduan (mis. "saya ingin tulis log harian"). Kolom
// sengaja TANPA contoh berangka: bila seluruh pesan ini kebetulan disalin & dikirim
// balik tanpa diisi, tidak ada baris yang keliru terparse sebagai metrik.
func (g *WhatsAppGateway) SendLogTemplate(ctx context.Context, toPhone, patientName string) error {
	text := fmt.Sprintf(
		"📝 *Sehatiku — Template Log Harian*\n\n"+
			"Halo %s 👋\n"+
			"Salin pesan di bawah, isi nilainya, lalu kirim balik. Kosongkan yang tidak ada.\n\n"+
			"Gula: \n"+
			"Tensi: \n"+
			"Makan: \n"+
			"Stres: \n"+
			"Obat: \n"+
			"Olahraga: \n"+
			"Tidur: \n"+
			"Berat: \n\n"+
			"Cara isi: gula, olahraga, tidur, dan berat tulis angka; stres tulis angka 1-10; "+
			"tensi tulis sistolik garis miring diastolik; obat tulis ya atau tidak; makan tulis nama menunya 🙏\n\n"+
			"─────────────\n"+
			"*Contoh yang sudah diisi:*\n\n"+
			"Gula: 180\n"+
			"Tensi: 120/80\n"+
			"Makan: nasi goreng 1 piring\n"+
			"Stres: 4\n"+
			"Obat: ya\n"+
			"Olahraga: 30\n"+
			"Tidur: 7\n"+
			"Berat: 65",
		patientName,
	)
	if err := g.sendText(ctx, toPhone, text); err != nil {
		return fmt.Errorf("sending log template to %s: %w", toPhone, err)
	}
	g.Log.Info("wa log template sent", zap.String("to", toPhone))
	return nil
}

// SendHealthLogParseError mengirim panduan format pesan yang benar ketika
// bot tidak bisa mengenali metrik dari pesan yang dikirim pasien/pendamping.
func (g *WhatsAppGateway) SendHealthLogParseError(ctx context.Context, toPhone string) error {
	text := "❓ *Sehatiku*\n\nMaaf, kami belum bisa mengenali pesan Anda.\n\n" +
		"Gunakan format berikut untuk mencatat data:\n\n" +
		"🩸 Gula darah  : *gula 180*\n" +
		"💊 Tekanan darah: *tensi 120/80*\n" +
		"💊 Kepatuhan obat: *obat ya* atau *tidak minum obat*\n" +
		"🍚 Makanan      : *makan nasi goreng*\n" +
		"🚶 Olahraga     : *olahraga 30 menit*\n" +
		"😴 Tidur        : *tidur 7 jam*\n" +
		"😓 Stres        : *stres 3*\n" +
		"⚖️ Berat badan  : *berat 65 kg*\n\n" +
		"Balas dengan format di atas ya 🙏"
	if err := g.sendText(ctx, toPhone, text); err != nil {
		return fmt.Errorf("sending parse error guide to %s: %w", toPhone, err)
	}
	g.Log.Info("wa parse error guide sent", zap.String("to", toPhone))
	return nil
}

// SendHealthLogNotRegistered mengirim notifikasi bahwa nomor pengirim tidak ditemukan
// di sistem Sehatiku — kemungkinan salah nomor atau belum terdaftar.
func (g *WhatsAppGateway) SendHealthLogNotRegistered(ctx context.Context, toPhone string) error {
	text := "⚠️ *Sehatiku*\n\n" +
		"Nomor ini belum terdaftar di Sehatiku.\n\n" +
		"Jika Anda adalah pasien atau pendamping, silakan hubungi faskes Anda untuk mendaftarkan nomor ini 🏥"
	if err := g.sendText(ctx, toPhone, text); err != nil {
		return fmt.Errorf("sending not-registered notice to %s: %w", toPhone, err)
	}
	g.Log.Info("wa not-registered notice sent", zap.String("to", toPhone))
	return nil
}

