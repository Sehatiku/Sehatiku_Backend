package main

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"sehatiku-backend/internal/config"

	"go.uber.org/zap"
	"gorm.io/gorm"
)

// Tabel yang DIHAPUS — urutan mengikuti dependency FK (child → parent).
// faskes, nakes, patients TIDAK ada di sini (user data dipertahankan).
var tablesToTruncate = []string{
	"patient_notifications",       // child dari patients, consultations
	"consultations",               // child dari patients, nakes
	"notifications",               // child dari escalations & patients
	"device_push_tokens",          // child dari patients
	"escalations",                 // child dari risk_scores, patients, nakes, faskes
	"risk_scores",                 // child dari daily_features, model_versions, patients
	"daily_features",              // child dari patients
	"patient_clinical_baselines",  // child dari patients
	"lab_results",                 // child dari patients, nakes
	"health_logs",                 // child dari patients
	"model_versions",              // standalone governance table
}

func main() {
	cfg := config.NewViper()
	logger := config.NewLogger(cfg)
	db := config.ConnectDB(cfg, logger)

	// ── Banner ──────────────────────────────────────────────────────────────
	fmt.Println()
	fmt.Println("╔══════════════════════════════════════════════════════════╗")
	fmt.Println("║        SEHATIKU — Truncate DB (Keep Users)              ║")
	fmt.Println("╚══════════════════════════════════════════════════════════╝")
	fmt.Println()
	fmt.Println("Yang akan DIHAPUS:")
	for _, t := range tablesToTruncate {
		fmt.Printf("  ✗  %s\n", t)
	}
	fmt.Println()
	fmt.Println("Yang DIPERTAHANKAN:")
	fmt.Println("  ✓  faskes")
	fmt.Println("  ✓  nakes")
	fmt.Println("  ✓  patients")
	fmt.Println()

	// ── Konfirmasi ──────────────────────────────────────────────────────────
	fmt.Print("Ketik 'HAPUS' (kapital) untuk konfirmasi, atau Enter untuk batal: ")
	reader := bufio.NewReader(os.Stdin)
	input, _ := reader.ReadString('\n')
	input = strings.TrimSpace(input)

	if input != "HAPUS" {
		fmt.Println("Dibatalkan. Tidak ada perubahan.")
		os.Exit(0)
	}

	// ── Eksekusi dalam satu transaksi ────────────────────────────────────────
	fmt.Println()
	fmt.Println("Menjalankan truncate...")

	err := db.Transaction(func(tx *gorm.DB) error {
		for _, table := range tablesToTruncate {
			logger.Info("truncating table", zap.String("table", table))
			sql := fmt.Sprintf(`TRUNCATE TABLE "%s" RESTART IDENTITY CASCADE`, table)
			if result := tx.Exec(sql); result.Error != nil {
				return fmt.Errorf("gagal truncate tabel %s: %w", table, result.Error)
			}
		}
		return nil
	})

	if err != nil {
		logger.Fatal("truncate gagal, semua perubahan di-rollback", zap.Error(err))
	}

	// ── Verifikasi ───────────────────────────────────────────────────────────
	fmt.Println()
	fmt.Println("Verifikasi row count setelah truncate:")
	fmt.Printf("  %-35s %s\n", "Tabel", "Sisa Rows")
	fmt.Println("  " + strings.Repeat("─", 45))

	allTables := append([]string{"faskes", "nakes", "patients"}, tablesToTruncate...)
	for _, table := range allTables {
		var count int64
		db.Table(table).Count(&count)
		status := "✓"
		if count > 0 && table != "faskes" && table != "nakes" && table != "patients" {
			status = "✗ (masih ada data!)"
		}
		fmt.Printf("  %-35s %d  %s\n", table, count, status)
	}

	fmt.Println()
	fmt.Println("╔══════════════════════════════════════════════════════════╗")
	fmt.Println("║  Truncate selesai. faskes / nakes / patients tetap utuh ║")
	fmt.Println("╚══════════════════════════════════════════════════════════╝")
	fmt.Println()

	logger.Info("truncate-db selesai")
}
