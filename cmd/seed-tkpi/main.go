// cmd/seed-tkpi mengisi tabel `tkpi` dari db/seed/tkpi_details.csv (TKPI Kemenkes 2019).
// Idempoten: di-upsert berdasarkan `kode`, jadi aman dijalankan berulang.
// Jalankan dari root repo:  go run ./cmd/seed-tkpi
package main

import (
	"encoding/csv"
	"os"
	"strconv"
	"strings"

	"sehatiku-backend/internal/config"
	"sehatiku-backend/internal/entity"

	"go.uber.org/zap"
	"gorm.io/gorm/clause"
)

const csvPath = "db/seed/tkpi_details.csv"

func main() {
	cfg := config.NewViper()
	log := config.NewLogger(cfg)
	db := config.ConnectDB(cfg, log)

	f, err := os.Open(csvPath)
	if err != nil {
		log.Fatal("buka csv", zap.String("path", csvPath), zap.Error(err))
	}
	defer f.Close()

	r := csv.NewReader(f)
	r.FieldsPerRecord = -1 // jumlah kolom antar baris bisa berbeda
	rows, err := r.ReadAll()
	if err != nil {
		log.Fatal("baca csv", zap.Error(err))
	}
	if len(rows) < 2 {
		log.Fatal("csv kosong / tanpa data")
	}

	idx := map[string]int{}
	for i, h := range rows[0] {
		idx[strings.TrimSpace(h)] = i
	}
	get := func(row []string, name string) string {
		i, ok := idx[name]
		if !ok || i >= len(row) {
			return ""
		}
		return strings.TrimSpace(row[i])
	}
	num := func(s string) *float64 {
		if s == "" {
			return nil
		}
		v, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return nil
		}
		return &v
	}
	str := func(s string) *string {
		if s == "" {
			return nil
		}
		return &s
	}

	items := make([]entity.Tkpi, 0, len(rows)-1)
	for _, row := range rows[1:] {
		kode := get(row, "kode")
		if kode == "" {
			continue
		}
		items = append(items, entity.Tkpi{
			Kode:         kode,
			NamaBahan:    get(row, "nama_bahan"),
			JenisPangan:  str(get(row, "Jenis pangan")),
			Kelompok:     str(get(row, "Kelompok")),
			NamaInggris:  str(get(row, "Nama Inggris")),
			EnergiKkal:   num(get(row, "Energi (Energy)")),
			ProteinG:     num(get(row, "Protein")),
			LemakG:       num(get(row, "Lemak (Fat)")),
			KarbohidratG: num(get(row, "Karbohidrat (CHO)")),
			NatriumMg:    num(get(row, "Natrium (Na), Sodium")),
			KaliumMg:     num(get(row, "Kalium (K), Potassium")),
			KalsiumMg:    num(get(row, "Kalsium (Ca), Calcium")),
			BesiMg:       num(get(row, "Besi (Fe), Ferrum, Iron")),
			AirG:         num(get(row, "Air (Water)")),
			AbuG:         num(get(row, "Abu (Ash)")),
		})
	}

	// Upsert per `kode` (UpdateAll) supaya re-run = refresh data.
	res := db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "kode"}},
		UpdateAll: true,
	}).CreateInBatches(items, 200)
	if res.Error != nil {
		log.Fatal("seed tkpi", zap.Error(res.Error))
	}

	log.Sugar().Infof("TKPI seeded: %d bahan", len(items))
}
