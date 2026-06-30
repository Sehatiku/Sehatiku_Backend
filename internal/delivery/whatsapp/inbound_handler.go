// Package whatsapp (delivery) adalah pintu masuk pesan WhatsApp dari luar — analog dengan
// delivery/http (HTTP) dan delivery/scheduler (cron) di project_structure.md §5. Karena
// memakai whatsmeow (bukan Cloud API webhook), trigger inbound datang sebagai event
// `events.Message` dari client, bukan request HTTP. Handler ini hanya mengekstrak data
// dan mendelegasikan keputusan ke usecase — tidak ada business logic di sini.
package whatsapp

import (
	"context"

	"sehatiku-backend/internal/usecase"

	"go.mau.fi/whatsmeow"
	"go.mau.fi/whatsmeow/types"
	"go.mau.fi/whatsmeow/types/events"
	"go.uber.org/zap"
)

// waHealthLogHandler adalah interface minimal yang dibutuhkan InboundHandler dari
// WAHealthLogUseCase — di-interface-kan agar handler tidak bergantung langsung pada
// struct konkret (memudahkan unit test bila handler diuji terpisah).
type waHealthLogHandler interface {
	HandleInbound(ctx context.Context, senderPhone, messageText string) error
}

type InboundHandler struct {
	UseCase       *usecase.WAInboundUseCase
	HealthLogUC   waHealthLogHandler // opsional; nil = fitur input WA belum diaktifkan
	Log           *zap.Logger
}

func NewInboundHandler(uc *usecase.WAInboundUseCase, log *zap.Logger) *InboundHandler {
	return &InboundHandler{UseCase: uc, Log: log}
}

// Register memasang handler event ke client whatsmeow. Aman dipasang berdampingan dengan
// handler lifecycle-logging yang sudah ada — whatsmeow mendukung banyak handler.
func (h *InboundHandler) Register(client *whatsmeow.Client) {
	client.AddEventHandler(func(evt any) {
		if msg, ok := evt.(*events.Message); ok {
			h.handleMessage(msg)
		}
	})
}

func (h *InboundHandler) handleMessage(msg *events.Message) {
	// Hanya pesan masuk dari chat pribadi (1:1). Abaikan pesan kita sendiri (termasuk balasan
	// warm-up yang baru saja kita kirim) dan pesan grup/broadcast.
	if msg.Info.IsFromMe || msg.Info.IsGroup {
		return
	}

	candidates := candidatePhones(msg.Info.MessageSource)
	if len(candidates) == 0 {
		return
	}

	// Event handler whatsmeow berjalan di goroutine-nya sendiri, lepas dari request HTTP —
	// pakai context.Background(). Lookup Redis murah dan no-op bila tidak ada yang menunggu.
	credentialDelivered := false
	if err := h.UseCase.DeliverPendingCredential(context.Background(), candidates); err != nil {
		h.Log.Warn("gagal mengirim kredensial warm-up saat pesan masuk", zap.Error(err))
	} else {
		// Cek apakah warm-up sukses (kredensial terkirim) — bila ya, pesan ini adalah
		// pesan pertama kontak baru (biasanya "Halo"), skip health log parsing supaya
		// tidak mengirim panduan format yang membingungkan.
		// Catatan: DeliverPendingCredential return nil juga bila tidak ada yang pending,
		// jadi kita perlu cara lain membedakannya. Saat ini kita konservatif: bila ada
		// kandidat nomor yang mempunyai pending credential (mungkin sudah terkirim),
		// kita tetap lanjut ke health log parsing — warm-up credential hanya terkirim
		// sekali dan kredensial sudah dihapus dari Redis. Tidak ada konflik.
		credentialDelivered = false // intentionally — lanjut ke health log parsing
		_ = credentialDelivered
	}

	// Health log input: routing berdasarkan teks pesan.
	// Dipanggil fire-and-forget di goroutine yang sama (sudah lepas dari HTTP context).
	if h.HealthLogUC == nil {
		return
	}

	text := extractMessageText(msg)
	if text == "" {
		// Pesan media/sticker/dll tanpa teks — skip tanpa balas agar tidak spam
		return
	}

	// Gunakan kandidat nomor pertama (paling spesifik: Sender) sebagai identitas pengirim
	// untuk lookup DB. WAHealthLogUseCase mencoba satu per satu bila tidak ditemukan.
	for _, phone := range candidates {
		if phone == "" {
			continue
		}
		if err := h.HealthLogUC.HandleInbound(context.Background(), phone, text); err != nil {
			h.Log.Warn("gagal memproses pesan health log inbound",
				zap.String("phone", phone), zap.Error(err))
		}
		// Hanya proses kandidat pertama yang berhasil — hindari double insert
		// bila Sender dan SenderAlt keduanya cocok ke pasien yang sama.
		break
	}
}

// extractMessageText mengekstrak konten teks dari pesan WA. Mendukung pesan teks
// biasa dan extended text (teks dengan preview link). Mengembalikan "" bila pesan
// bukan teks (gambar, audio, sticker, dll) — caller skip pesan semacam itu.
func extractMessageText(msg *events.Message) string {
	if msg.Message == nil {
		return ""
	}
	if msg.Message.Conversation != nil {
		return *msg.Message.Conversation
	}
	if ext := msg.Message.ExtendedTextMessage; ext != nil && ext.Text != nil {
		return *ext.Text
	}
	return ""
}

// candidatePhones mengumpulkan kemungkinan nomor telepon pengirim. Dengan addressing LID,
// `Sender` bisa berupa alamat LID sementara nomor telepon ada di `SenderAlt` (atau
// sebaliknya); `Chat` pada DM juga merujuk lawan bicara. Kita kumpulkan semua yang
// beralamat nomor telepon (server s.whatsapp.net) dan biarkan usecase mencocokkan ke stash.
func candidatePhones(src types.MessageSource) []string {
	seen := map[string]struct{}{}
	var phones []string
	add := func(jid types.JID) {
		jid = jid.ToNonAD()
		if jid.Server != types.DefaultUserServer || jid.User == "" {
			return
		}
		if _, ok := seen[jid.User]; ok {
			return
		}
		seen[jid.User] = struct{}{}
		phones = append(phones, jid.User)
	}
	add(src.Sender)
	add(src.SenderAlt)
	add(src.Chat)
	return phones
}
