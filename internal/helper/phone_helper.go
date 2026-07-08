package helper

import "strings"

// MaskPhone menyamarkan sebagian nomor telepon untuk logging — kita tidak pernah log
// nomor penuh (be_implementation §8). Contoh: "62812345678" -> "628*****678".
func MaskPhone(raw string) string {
	digits := NormalizePhoneID(raw)
	if len(digits) <= 6 {
		return "***"
	}
	return digits[:3] + strings.Repeat("*", len(digits)-6) + digits[len(digits)-3:]
}

// NormalizePhoneID menormalkan nomor telepon Indonesia ke format internasional
// telanjang (hanya digit, tanpa '+' atau pemisah) yang dibutuhkan whatsmeow saat
// membangun JID. Nomor di DB biasanya tersimpan dalam format lokal (mis. 0812345678),
// sedangkan WhatsApp membutuhkan 62812345678; tanpa normalisasi ini usync device-list
// query akan timeout dan pengiriman pesan gagal.
//
// Contoh: "0812345678" -> "62812345678", "+62 812-345-678" -> "62812345678",
// "628123" -> "628123", "8123" -> "628123", "" -> "".
func NormalizePhoneID(raw string) string {
	var b strings.Builder
	for _, r := range raw {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	digits := b.String()

	switch {
	case digits == "":
		return ""
	case strings.HasPrefix(digits, "62"):
		return digits
	case strings.HasPrefix(digits, "0"):
		return "62" + digits[1:]
	case strings.HasPrefix(digits, "8"):
		return "62" + digits
	default:
		return digits
	}
}
