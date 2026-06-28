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

type InboundHandler struct {
	UseCase *usecase.WAInboundUseCase
	Log     *zap.Logger
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
	if err := h.UseCase.DeliverPendingCredential(context.Background(), candidates); err != nil {
		h.Log.Warn("gagal mengirim kredensial warm-up saat pesan masuk", zap.Error(err))
	}
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
