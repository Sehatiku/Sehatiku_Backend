package entity

// Tkpi adalah satu baris referensi gizi dari Tabel Komposisi Pangan Indonesia
// (TKPI, Kemenkes 2019). Tabel ini bersifat REFERENSI/audit: sumber kebenaran konversi
// teks makanan -> gram tetap di ML service (NER + fuzzy match). Backend/dashboard dapat
// melookup gizi dari tabel ini bila perlu. Di-seed via `cmd/seed-tkpi`.
type Tkpi struct {
	Kode         string   `gorm:"column:kode;primaryKey"`
	NamaBahan    string   `gorm:"column:nama_bahan"`
	JenisPangan  *string  `gorm:"column:jenis_pangan"`
	Kelompok     *string  `gorm:"column:kelompok"`
	NamaInggris  *string  `gorm:"column:nama_inggris"`
	EnergiKkal   *float64 `gorm:"column:energi_kkal"`
	ProteinG     *float64 `gorm:"column:protein_g"`
	LemakG       *float64 `gorm:"column:lemak_g"`
	KarbohidratG *float64 `gorm:"column:karbohidrat_g"`
	NatriumMg    *float64 `gorm:"column:natrium_mg"`
	KaliumMg     *float64 `gorm:"column:kalium_mg"`
	KalsiumMg    *float64 `gorm:"column:kalsium_mg"`
	BesiMg       *float64 `gorm:"column:besi_mg"`
	AirG         *float64 `gorm:"column:air_g"`
	AbuG         *float64 `gorm:"column:abu_g"`
}

func (Tkpi) TableName() string { return "tkpi" }
