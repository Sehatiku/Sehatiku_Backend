-- 000015_tkpi.up.sql
-- Tabel referensi gizi TKPI (Tabel Komposisi Pangan Indonesia, Kemenkes 2019, ~1148 bahan).
-- Konversi teks makanan -> gram tetap dilakukan ML service (NER + fuzzy match atas CSV);
-- tabel ini untuk REFERENSI / audit / lookup oleh backend & dashboard. Di-seed via
-- `go run ./cmd/seed-tkpi` dari db/seed/tkpi_details.csv.

BEGIN;

CREATE TABLE tkpi (
    kode          TEXT PRIMARY KEY,
    nama_bahan    TEXT NOT NULL,
    jenis_pangan  TEXT,
    kelompok      TEXT,
    nama_inggris  TEXT,
    energi_kkal   NUMERIC,   -- Energi (Energy), kkal per 100 g
    protein_g     NUMERIC,
    lemak_g       NUMERIC,   -- Lemak (Fat)
    karbohidrat_g NUMERIC,   -- Karbohidrat (CHO)
    natrium_mg    NUMERIC,   -- Natrium (Na), Sodium
    kalium_mg     NUMERIC,
    kalsium_mg    NUMERIC,
    besi_mg       NUMERIC,
    air_g         NUMERIC,
    abu_g         NUMERIC,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Bantu pencarian nama bahan case-insensitive.
CREATE INDEX idx_tkpi_nama_bahan_lower ON tkpi (lower(nama_bahan));

COMMIT;
