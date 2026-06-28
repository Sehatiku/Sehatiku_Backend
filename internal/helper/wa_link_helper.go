package helper

import (
	"fmt"
	"net/url"

	"sehatiku-backend/internal/entity"
)

// BuildWAMeLink membangun link "click-to-chat" wa.me yang membuat penerima MENGHUBUNGI
// bot lebih dulu. Ini inti dari alur warm-up: WhatsApp memblokir pesan keluar ke kontak
// dingin (error 463), jadi penerima harus mengirim pesan ke bot terlebih dahulu —
// setelah itu balasan kredensial dari bot tidak lagi dianggap cold-reachout.
//
// botPhone adalah nomor WA bot dalam format internasional telanjang (mis. "62812345678",
// dari WhatsAppGateway.BotPhone()). prefilledText adalah teks yang sudah terisi di kolom
// chat penerima saat link dibuka. Bila botPhone kosong (device belum dipasangkan),
// mengembalikan string kosong supaya pemanggil bisa menandai link tidak tersedia.
func BuildWAMeLink(botPhone, prefilledText string) string {
	if botPhone == "" {
		return ""
	}
	link := "https://wa.me/" + botPhone
	if prefilledText != "" {
		link += "?text=" + url.QueryEscape(prefilledText)
	}
	return link
}

// BuildWarmupInviteText menyusun teks undangan aktivasi yang dikirim faskes ke penerima
// (pasien/pendamping/nakes). Teks ini menjadi `text=` pada link wa.me langsung-ke-penerima
// (lihat BuildDirectInviteLink): faskes klik link → chat ke penerima terbuka dengan teks ini
// sudah terisi → faskes tekan kirim. Penerima lalu tap botLink di dalam pesan → kirim pesan
// ke bot → bot menghangat dan otomatis membalas kredensial.
//
// KEAMANAN: teks ini SENGAJA tidak memuat password. Hanya username + link warm-up bot;
// password tetap hanya jalan bot→penerima setelah warm-up. Faskes tetap memegang password
// di response registrasi sebagai cadangan terjamin.
//
// botLink adalah hasil BuildWAMeLink (link ke bot). Bila botLink kosong (bot belum
// dipasangkan), mengembalikan string kosong supaya pemanggil tidak menyusun undangan tanpa
// link warm-up yang bisa ditindaklanjuti.
func BuildWarmupInviteText(role, recipientName, patientName, username, botLink string) string {
	if botLink == "" {
		return ""
	}

	switch role {
	case entity.RecipientRoleCompanion:
		return fmt.Sprintf(
			"Halo Bapak/Ibu %s 🙏\n\n"+
				"Anda terdaftar sebagai pendamping %s di Sehatiku.\n\n"+
				"Untuk mengaktifkan, buka tautan ini lalu tekan tombol kirim:\n%s\n\n"+
				"Setelah Anda mengirim pesan, detail akun akan otomatis dikirim oleh Sehatiku lewat WhatsApp ini.",
			recipientName, patientName, botLink,
		)
	default: // patient | nakes — teks sama (pemegang akun)
		return fmt.Sprintf(
			"Halo Bapak/Ibu %s 🙏\n\n"+
				"Akun Sehatiku Anda sudah dibuat.\n"+
				"Username: %s\n\n"+
				"Untuk mengaktifkan dan menerima password, buka tautan ini lalu tekan tombol kirim:\n%s\n\n"+
				"Setelah Anda mengirim pesan, password akan otomatis dikirim oleh Sehatiku lewat WhatsApp ini.",
			recipientName, username, botLink,
		)
	}
}

// BuildDirectInviteLink membangun link wa.me yang menunjuk ke nomor PENERIMA sendiri
// (bukan bot), dengan teks undangan aktivasi sudah terisi. Faskes cukup mengklik link ini:
// WhatsApp faskes langsung membuka chat ke pasien/pendamping/nakes dengan pesan siap kirim —
// menggantikan alur salin-tempel manual.
//
// recipientPhone dinormalkan ke format internasional telanjang (lihat NormalizePhoneID).
// Mengembalikan string kosong bila teks undangan kosong (botLink kosong / bot belum
// dipasangkan) atau recipientPhone kosong — pada kasus itu faskes menyampaikan kredensial
// manual lewat field `credentials`.
func BuildDirectInviteLink(recipientPhone, role, recipientName, patientName, username, botLink string) string {
	text := BuildWarmupInviteText(role, recipientName, patientName, username, botLink)
	if text == "" {
		return ""
	}
	return BuildWAMeLink(NormalizePhoneID(recipientPhone), text)
}
