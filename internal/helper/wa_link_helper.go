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

// BuildWarmupShareMessage menyusun teks SIAP-TEMPEL yang diteruskan faskes ke penerima
// (pasien/pendamping/nakes) lewat kanal pribadi mereka — telepon, WA pribadi staf, atau
// SMS manual (alur faskes-mediated). Ini menjembatani gap "link nyangkut di layar faskes":
// faskes tinggal salin-bagikan, penerima tap link → kirim pesan → bot menghangat dan
// otomatis membalas kredensial.
//
// KEAMANAN: pesan ini SENGAJA tidak memuat password. Hanya username + link aktivasi;
// password tetap hanya jalan bot→penerima setelah warm-up. Faskes tetap memegang password
// di response registrasi sebagai cadangan terjamin.
//
// link adalah hasil BuildWAMeLink. Bila link kosong (bot belum dipasangkan), mengembalikan
// string kosong supaya pemanggil tidak menyertakan pesan tanpa link yang bisa ditindaklanjuti.
func BuildWarmupShareMessage(role, recipientName, patientName, username, link string) string {
	if link == "" {
		return ""
	}

	switch role {
	case entity.RecipientRoleCompanion:
		return fmt.Sprintf(
			"Halo Bapak/Ibu %s 🙏\n\n"+
				"Anda terdaftar sebagai pendamping %s di Sehatiku.\n\n"+
				"Untuk mengaktifkan, buka tautan ini lalu tekan tombol kirim:\n%s\n\n"+
				"Setelah Anda mengirim pesan, detail akun akan otomatis dikirim oleh Sehatiku lewat WhatsApp ini.",
			recipientName, patientName, link,
		)
	default: // patient | nakes — teks sama (pemegang akun)
		return fmt.Sprintf(
			"Halo Bapak/Ibu %s 🙏\n\n"+
				"Akun Sehatiku Anda sudah dibuat.\n"+
				"Username: %s\n\n"+
				"Untuk mengaktifkan dan menerima password, buka tautan ini lalu tekan tombol kirim:\n%s\n\n"+
				"Setelah Anda mengirim pesan, password akan otomatis dikirim oleh Sehatiku lewat WhatsApp ini.",
			recipientName, username, link,
		)
	}
}
