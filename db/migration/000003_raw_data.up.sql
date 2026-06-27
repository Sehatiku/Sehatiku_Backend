-- 000003_raw_data.up.sql
-- Cluster 2: Data Mentah (insert-only) — health_logs, lab_results

BEGIN;

-- ---------- health_logs (event stream input harian WA) ----------
-- Insert-only: satu baris per pengukuran. measured_at = waktu asli pengukuran.
-- Tipe nilai fleksibel sesuai metric_type (numeric / text / jsonb).
CREATE TABLE health_logs (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    patient_id    UUID          NOT NULL REFERENCES patients(id) ON DELETE RESTRICT,
    logged_by     health_logger NOT NULL DEFAULT 'patient',
    metric_type   health_metric NOT NULL,
    value_numeric NUMERIC,                 -- glucose, systolic/diastolic, sleep_hours, dst
    value_text    TEXT,                    -- catatan makanan mentah, dll
    value_jsonb   JSONB,                   -- hasil NER makanan / payload terstruktur
    measured_at   TIMESTAMPTZ   NOT NULL,
    source        log_source    NOT NULL DEFAULT 'whatsapp',
    created_at    TIMESTAMPTZ   NOT NULL DEFAULT now()
);
CREATE INDEX idx_health_logs_patient_time ON health_logs(patient_id, measured_at);
CREATE INDEX idx_health_logs_metric       ON health_logs(patient_id, metric_type, measured_at);

-- ---------- lab_results (lab faskes, point-in-time) ----------
-- Insert-only: satu baris per (patient, lab_type, result_date), independen per jenis lab.
-- result_date DESC mendukung as-of join (WHERE result_date <= T ORDER BY ... DESC).
CREATE TABLE lab_results (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    patient_id    UUID        NOT NULL REFERENCES patients(id) ON DELETE RESTRICT,
    recorded_by   UUID        REFERENCES nakes(id) ON DELETE SET NULL,
    lab_type      lab_type    NOT NULL,
    value_numeric NUMERIC     NOT NULL,
    unit          TEXT        NOT NULL,
    result_date   DATE        NOT NULL,
    source        lab_source  NOT NULL DEFAULT 'faskes',
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_lab_results_asof ON lab_results(patient_id, lab_type, result_date DESC);

COMMIT;
