-- 000016_baseline_audit_columns.up.sql
-- Menjadikan patient_clinical_baselines bisa dipakai sebagai LOG progress: faskes mencatat
-- versi baseline baru dari waktu ke waktu (insert-only, satu baris per pencatatan). Dua
-- kolom audit ditambah supaya tiap entri bisa ditelusuri:
--   - recorded_by_nakes_id : nakes yang mencatat (nullable; baris lama dari registrasi NULL)
--   - notes                : catatan/alasan bebas (nullable)
-- Nullable wajib karena baris baseline lama (dibuat saat registrasi) tidak memilikinya.

BEGIN;

ALTER TABLE patient_clinical_baselines
    ADD COLUMN IF NOT EXISTS recorded_by_nakes_id UUID REFERENCES nakes(id) ON DELETE SET NULL,
    ADD COLUMN IF NOT EXISTS notes                TEXT;

COMMIT;
