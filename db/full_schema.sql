-- full_schema.sql
-- Sehatiku MVP — schema lengkap untuk bootstrap database baru.
--
-- File ini adalah konsolidasi dari migration 000001-000007.
-- Untuk development/staging: jalankan langsung via psql.
-- Untuk production: tetap gunakan golang-migrate dengan file bernomor di db/migration/.
--
-- Urutan: extensions → enums → faskes → nakes → patients → baselines
--         → health_logs → lab_results → model_versions → daily_features
--         → risk_scores → escalations → notifications

BEGIN;

-- =============================================================================
-- EXTENSIONS
-- =============================================================================

CREATE EXTENSION IF NOT EXISTS pgcrypto;

-- =============================================================================
-- ENUM TYPES
-- =============================================================================

-- Tenancy & identitas
CREATE TYPE faskes_type    AS ENUM ('puskesmas', 'klinik');
CREATE TYPE entity_status  AS ENUM ('active', 'inactive');
CREATE TYPE nakes_role     AS ENUM ('dokter', 'kader', 'admin');
CREATE TYPE patient_sex    AS ENUM ('male', 'female');
CREATE TYPE disease_type   AS ENUM ('diabetes_t2', 'hypertension', 'both');

-- Data mentah
CREATE TYPE health_logger  AS ENUM ('patient', 'companion');
CREATE TYPE health_metric  AS ENUM (
    'glucose', 'bp', 'med_adherence', 'food',
    'activity', 'sleep', 'stress', 'smoking', 'alcohol'
);
CREATE TYPE log_source     AS ENUM ('whatsapp', 'sms', 'web', 'app');
CREATE TYPE lab_type       AS ENUM (
    'hba1c', 'ldl', 'hdl', 'triglyceride', 'total_chol',
    'egfr', 'uacr', 'bmi', 'waist_circumference',
    'baseline_systolic', 'baseline_diastolic'
);
CREATE TYPE lab_source     AS ENUM ('faskes', 'manual');

-- Turunan ML
CREATE TYPE risk_status    AS ENUM ('aman', 'waswas', 'bahaya');
CREATE TYPE scoring_mode   AS ENUM ('rule_based', 'cohort');

-- Aksi & komunikasi
CREATE TYPE escalation_tier     AS ENUM ('acute_today', 'trend_this_week');
CREATE TYPE comm_channel        AS ENUM ('whatsapp', 'sms');
CREATE TYPE escalation_status   AS ENUM ('sent', 'viewed', 'acted', 'dismissed');
CREATE TYPE escalation_feedback AS ENUM ('accurate', 'inaccurate');
CREATE TYPE recipient_role      AS ENUM ('patient', 'companion', 'nakes');
CREATE TYPE message_type        AS ENUM (
    'credential_blast', 'escalation', 'recommendation',
    'daily_prompt', 'system'
);
CREATE TYPE notification_status AS ENUM ('queued', 'sent', 'delivered', 'failed');

-- Governance ML
CREATE TYPE model_type AS ENUM ('nutrition_match', 'cohort_risk');

-- =============================================================================
-- CLUSTER 1: TENANCY & IDENTITAS
-- =============================================================================

CREATE TABLE faskes (
    id            UUID          PRIMARY KEY DEFAULT gen_random_uuid(),
    name          TEXT          NOT NULL,
    type          faskes_type   NOT NULL,
    address       TEXT,
    region        TEXT,
    username      TEXT          NOT NULL,
    password_hash TEXT          NOT NULL,
    phone_number  TEXT          NOT NULL,
    status        entity_status NOT NULL DEFAULT 'active',
    created_at    TIMESTAMPTZ   NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ   NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX idx_faskes_username ON faskes(username);

CREATE TABLE nakes (
    id            UUID          PRIMARY KEY DEFAULT gen_random_uuid(),
    faskes_id     UUID          NOT NULL REFERENCES faskes(id) ON DELETE RESTRICT,
    username      TEXT          NOT NULL,
    password_hash TEXT          NOT NULL,
    full_name     TEXT          NOT NULL,
    role          nakes_role    NOT NULL DEFAULT 'dokter',
    nik           TEXT          NOT NULL,
    alamat        TEXT          NOT NULL,
    phone_number  TEXT          NOT NULL,
    status        entity_status NOT NULL DEFAULT 'active',
    enrolled_at   TIMESTAMPTZ   NOT NULL DEFAULT now(),
    created_at    TIMESTAMPTZ   NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ   NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX idx_nakes_username ON nakes(username);
CREATE INDEX        idx_nakes_faskes   ON nakes(faskes_id);

CREATE TABLE patients (
    id                UUID          PRIMARY KEY DEFAULT gen_random_uuid(),
    faskes_id         UUID          NOT NULL REFERENCES faskes(id) ON DELETE RESTRICT,
    assigned_nakes_id UUID          NOT NULL REFERENCES nakes(id)  ON DELETE RESTRICT,
    username          TEXT          NOT NULL,
    password_hash     TEXT          NOT NULL,
    full_name         TEXT          NOT NULL,
    nik               TEXT          NOT NULL,
    alamat            TEXT          NOT NULL,
    phone_number      TEXT          NOT NULL,
    companion_name    TEXT,
    companion_phone   TEXT,
    date_of_birth     DATE,
    sex               patient_sex,
    disease_type      disease_type  NOT NULL,
    enrolled_at       TIMESTAMPTZ   NOT NULL DEFAULT now(),
    status            entity_status NOT NULL DEFAULT 'active',
    created_at        TIMESTAMPTZ   NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ   NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX idx_patients_username ON patients(username);
CREATE INDEX        idx_patients_faskes   ON patients(faskes_id);
CREATE INDEX        idx_patients_nakes    ON patients(assigned_nakes_id);

-- Baseline klinis pasien (tren grafik per kunjungan nakes)
CREATE TABLE patient_clinical_baselines (
    id                  UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    patient_id          UUID        NOT NULL REFERENCES patients(id) ON DELETE CASCADE,
    recorded_at         DATE        NOT NULL DEFAULT CURRENT_DATE,
    hba1c               NUMERIC(5,2),
    lipid_profile       JSONB,
    egfr                NUMERIC(7,2),
    uacr                NUMERIC(7,2),
    bmi                 NUMERIC(5,2),
    waist_circumference NUMERIC(5,2),
    systolic_bp         INTEGER,
    diastolic_bp        INTEGER,
    notes               TEXT,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_patient_baselines_patient ON patient_clinical_baselines(patient_id);
CREATE INDEX idx_patient_baselines_date    ON patient_clinical_baselines(recorded_at);

-- =============================================================================
-- CLUSTER 2: DATA MENTAH (insert-only)
-- =============================================================================

-- health_logs: event stream input harian WhatsApp / mobile app.
-- Satu baris per pengukuran; tipe nilai fleksibel via value_numeric / value_text / value_jsonb.
CREATE TABLE health_logs (
    id            UUID          PRIMARY KEY DEFAULT gen_random_uuid(),
    patient_id    UUID          NOT NULL REFERENCES patients(id) ON DELETE RESTRICT,
    logged_by     health_logger NOT NULL DEFAULT 'patient',
    metric_type   health_metric NOT NULL,
    value_numeric NUMERIC,
    value_text    TEXT,
    value_jsonb   JSONB,
    measured_at   TIMESTAMPTZ   NOT NULL,
    source        log_source    NOT NULL DEFAULT 'whatsapp',
    created_at    TIMESTAMPTZ   NOT NULL DEFAULT now()
);
CREATE INDEX idx_health_logs_patient_time ON health_logs(patient_id, measured_at);
CREATE INDEX idx_health_logs_metric       ON health_logs(patient_id, metric_type, measured_at);

-- lab_results: hasil lab faskes (point-in-time, as-of join).
-- Query: WHERE patient_id = ? AND result_date <= T ORDER BY result_date DESC.
CREATE TABLE lab_results (
    id            UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
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

-- =============================================================================
-- CLUSTER 3 + 5 (ML): MODEL_VERSIONS, DAILY_FEATURES, RISK_SCORES
-- =============================================================================

-- model_versions dibuat lebih dulu — di-referensikan oleh risk_scores.
CREATE TABLE model_versions (
    id         UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    model_type model_type  NOT NULL,
    version    TEXT        NOT NULL,
    metrics    JSONB,
    is_active  BOOLEAN     NOT NULL DEFAULT false,
    notes      TEXT,
    trained_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX idx_model_versions_type_version ON model_versions(model_type, version);
-- Hanya satu versi aktif per model_type pada satu waktu.
CREATE UNIQUE INDEX idx_model_versions_one_active ON model_versions(model_type) WHERE is_active;

-- daily_features: vektor fitur typed 1 baris/pasien/hari, hasil cron batch harian.
-- SATU-SATUNYA sumber input untuk XGBoost — kolom typed, tidak ada parsing JSONB saat scoring.
CREATE TABLE daily_features (
    id                      UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
    patient_id              UUID        NOT NULL REFERENCES patients(id) ON DELETE RESTRICT,
    feature_date            DATE        NOT NULL,
    glucose_avg             NUMERIC,
    glucose_max             NUMERIC,
    systolic_avg            NUMERIC,
    diastolic_avg           NUMERIC,
    med_adherence_rate      NUMERIC,
    total_sugar_g           NUMERIC,
    total_sodium_mg         NUMERIC,
    total_kcal              NUMERIC,
    activity_minutes        NUMERIC,
    sleep_hours             NUMERIC,
    stress_level            SMALLINT,
    smoking_flag            BOOLEAN,
    alcohol_flag            BOOLEAN,
    glucose_roll7_mean      NUMERIC,
    glucose_roll7_std       NUMERIC,
    glucose_slope7          NUMERIC,
    deviation_from_baseline NUMERIC,
    logging_streak          INTEGER,
    days_since_last_lab     INTEGER,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX idx_daily_features_patient_date ON daily_features(patient_id, feature_date);

-- risk_scores: output model, insert-only.
-- model_version_id NULL bila skor murni rule-based.
CREATE TABLE risk_scores (
    id               UUID         PRIMARY KEY DEFAULT gen_random_uuid(),
    patient_id       UUID         NOT NULL REFERENCES patients(id)       ON DELETE RESTRICT,
    daily_feature_id UUID         NOT NULL REFERENCES daily_features(id) ON DELETE RESTRICT,
    model_version_id UUID         REFERENCES model_versions(id)          ON DELETE SET NULL,
    score            SMALLINT     NOT NULL CHECK (score BETWEEN 0 AND 100),
    status           risk_status  NOT NULL,
    scoring_mode     scoring_mode NOT NULL,
    top_factors      JSONB,
    triggered_rule   TEXT,
    scored_at        TIMESTAMPTZ  NOT NULL DEFAULT now(),
    created_at       TIMESTAMPTZ  NOT NULL DEFAULT now()
);
CREATE INDEX idx_risk_scores_patient_time ON risk_scores(patient_id, scored_at DESC);

-- =============================================================================
-- CLUSTER 4: AKSI & KOMUNIKASI
-- =============================================================================

-- escalations: peristiwa klinis + feedback label emas nakes.
-- Satu eskalasi bisa memicu banyak notifications.
CREATE TABLE escalations (
    id                UUID                PRIMARY KEY DEFAULT gen_random_uuid(),
    patient_id        UUID                NOT NULL REFERENCES patients(id)    ON DELETE RESTRICT,
    risk_score_id     UUID                NOT NULL REFERENCES risk_scores(id) ON DELETE RESTRICT,
    faskes_id         UUID                NOT NULL REFERENCES faskes(id)      ON DELETE RESTRICT,
    assigned_nakes_id UUID                NOT NULL REFERENCES nakes(id)       ON DELETE RESTRICT,
    tier              escalation_tier     NOT NULL,
    channel           comm_channel        NOT NULL DEFAULT 'whatsapp',
    status            escalation_status   NOT NULL DEFAULT 'sent',
    sent_at           TIMESTAMPTZ         NOT NULL DEFAULT now(),
    viewed_at         TIMESTAMPTZ,
    acted_at          TIMESTAMPTZ,
    feedback          escalation_feedback,
    feedback_by       UUID                REFERENCES nakes(id) ON DELETE SET NULL,
    feedback_at       TIMESTAMPTZ,
    created_at        TIMESTAMPTZ         NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ         NOT NULL DEFAULT now()
);
-- Antrean prioritas dashboard faskes
CREATE INDEX idx_escalations_queue ON escalations(faskes_id, status, tier, sent_at DESC);
-- Query label training: WHERE feedback IS NOT NULL
CREATE INDEX idx_escalations_label ON escalations(feedback) WHERE feedback IS NOT NULL;

-- notifications: transport WA/SMS keluar, mendukung audit + retry.
-- patient_id / nakes_id nullable; constraint memastikan penerima konsisten dengan recipient_role.
CREATE TABLE notifications (
    id                  UUID                PRIMARY KEY DEFAULT gen_random_uuid(),
    patient_id          UUID                REFERENCES patients(id)    ON DELETE SET NULL,
    nakes_id            UUID                REFERENCES nakes(id)       ON DELETE SET NULL,
    escalation_id       UUID                REFERENCES escalations(id) ON DELETE SET NULL,
    recipient_phone     TEXT                NOT NULL,
    recipient_role      recipient_role      NOT NULL,
    message_type        message_type        NOT NULL,
    channel             comm_channel        NOT NULL DEFAULT 'whatsapp',
    payload             JSONB,
    status              notification_status NOT NULL DEFAULT 'queued',
    provider_message_id TEXT,
    error_reason        TEXT,
    retry_count         INTEGER             NOT NULL DEFAULT 0,
    queued_at           TIMESTAMPTZ         NOT NULL DEFAULT now(),
    sent_at             TIMESTAMPTZ,
    delivered_at        TIMESTAMPTZ,
    created_at          TIMESTAMPTZ         NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ         NOT NULL DEFAULT now(),
    CONSTRAINT chk_recipient_target CHECK (
        (recipient_role = 'nakes'  AND nakes_id IS NOT NULL) OR
        (recipient_role IN ('patient', 'companion') AND patient_id IS NOT NULL) OR
        (message_type = 'system')
    )
);
CREATE INDEX idx_notifications_retry   ON notifications(status, retry_count) WHERE status = 'failed';
CREATE INDEX idx_notifications_patient ON notifications(patient_id);
CREATE INDEX idx_notifications_nakes   ON notifications(nakes_id);

COMMIT;
