-- 000004_ml.up.sql
-- Cluster 5 (governance) + Cluster 3 (turunan ML): model_versions, daily_features, risk_scores
-- model_versions dibuat lebih dulu karena risk_scores mereferensikannya.

BEGIN;

-- ---------- model_versions (governance & drift) ----------
CREATE TABLE model_versions (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    model_type model_type  NOT NULL,
    version    TEXT        NOT NULL,
    metrics    JSONB,                       -- precision/recall/auc per kelas
    is_active  BOOLEAN     NOT NULL DEFAULT false,
    notes      TEXT,
    trained_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX idx_model_versions_type_version ON model_versions(model_type, version);
-- Hanya satu versi aktif per model_type pada satu waktu.
CREATE UNIQUE INDEX idx_model_versions_one_active
    ON model_versions(model_type) WHERE is_active;

-- ---------- daily_features (typed, 1 baris/pasien/hari, hasil cron batch) ----------
CREATE TABLE daily_features (
    id                      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    patient_id              UUID        NOT NULL REFERENCES patients(id) ON DELETE RESTRICT,
    feature_date            DATE        NOT NULL,
    -- agregat fisiologis
    glucose_avg             NUMERIC,
    glucose_max             NUMERIC,
    systolic_avg            NUMERIC,
    diastolic_avg           NUMERIC,
    med_adherence_rate      NUMERIC,
    -- agregat gaya hidup
    total_sugar_g           NUMERIC,
    total_sodium_mg         NUMERIC,
    total_kcal              NUMERIC,
    activity_minutes        NUMERIC,
    sleep_hours             NUMERIC,
    stress_level            SMALLINT,
    smoking_flag            BOOLEAN,
    alcohol_flag            BOOLEAN,
    -- fitur rolling / temporal
    glucose_roll7_mean      NUMERIC,
    glucose_roll7_std       NUMERIC,
    glucose_slope7          NUMERIC,
    deviation_from_baseline NUMERIC,
    logging_streak          INTEGER,
    days_since_last_lab     INTEGER,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX idx_daily_features_patient_date ON daily_features(patient_id, feature_date);

-- ---------- risk_scores (output model, insert-only) ----------
CREATE TABLE risk_scores (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    patient_id       UUID         NOT NULL REFERENCES patients(id)       ON DELETE RESTRICT,
    daily_feature_id UUID         NOT NULL REFERENCES daily_features(id) ON DELETE RESTRICT,
    model_version_id UUID         REFERENCES model_versions(id) ON DELETE SET NULL, -- NULL bila rule_based
    score            SMALLINT     NOT NULL CHECK (score BETWEEN 0 AND 100),
    status           risk_status  NOT NULL,
    scoring_mode     scoring_mode NOT NULL,
    top_factors      JSONB,                  -- [{feature, shap_value, direction}]
    triggered_rule   TEXT,                   -- diisi bila scoring_mode = rule_based
    scored_at        TIMESTAMPTZ  NOT NULL DEFAULT now(),
    created_at       TIMESTAMPTZ  NOT NULL DEFAULT now()
);
CREATE INDEX idx_risk_scores_patient_time ON risk_scores(patient_id, scored_at DESC);

COMMIT;
