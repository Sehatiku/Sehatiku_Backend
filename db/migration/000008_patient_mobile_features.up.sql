-- 000008_patient_mobile_features.up.sql
-- Fitur Mobile Patient App: info profesional nakes, catatan harian (records), konsultasi.
-- Dijalankan setelah 000007_faskes_auth.up.sql.

BEGIN;

-- nakes: tambah kolom informasi profesional (nullable — diisi terpisah oleh admin faskes)
ALTER TABLE nakes ADD COLUMN IF NOT EXISTS specialization text;
ALTER TABLE nakes ADD COLUMN IF NOT EXISTS hospital       text;
ALTER TABLE nakes ADD COLUMN IF NOT EXISTS schedule       jsonb;

-- health_metric enum: tambah 'weight' untuk pencatatan berat badan harian pasien via records endpoint
ALTER TYPE health_metric ADD VALUE IF NOT EXISTS 'weight';

-- consultations: keluhan pasien ke dokter penanggung jawab
CREATE TABLE IF NOT EXISTS consultations (
    id          uuid        PRIMARY KEY DEFAULT gen_random_uuid(),
    patient_id  uuid        NOT NULL REFERENCES patients(id),
    complaint   text        NOT NULL,
    status      varchar(20) NOT NULL DEFAULT 'open',
    created_at  timestamptz NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_consultations_patient ON consultations(patient_id, created_at DESC);

COMMIT;
