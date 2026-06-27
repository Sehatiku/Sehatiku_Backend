-- 000002_tenancy.up.sql
-- Cluster 1: Tenancy & Identitas — faskes, nakes, patients

BEGIN;

CREATE TABLE faskes (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name             TEXT          NOT NULL,
    type             faskes_type   NOT NULL,
    address          TEXT,
    region           TEXT,
    status           entity_status NOT NULL DEFAULT 'active',
    created_at       TIMESTAMPTZ   NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ   NOT NULL DEFAULT now()
);

CREATE TABLE nakes (
    id            UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    faskes_id     UUID          NOT NULL REFERENCES faskes(id) ON DELETE RESTRICT,
    username      TEXT          NOT NULL,
    password_hash TEXT          NOT NULL,
    full_name     TEXT          NOT NULL,
    role          nakes_role    NOT NULL DEFAULT 'dokter',
    nik          TEXT          NOT NULL,
    alamat       TEXT          NOT NULL,
    phone_number  TEXT          NOT NULL,
    status        entity_status NOT NULL DEFAULT 'active',
    enrolled_at   TIMESTAMPTZ   NOT NULL DEFAULT now(),
    created_at    TIMESTAMPTZ   NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ   NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX idx_nakes_username ON nakes(username);
CREATE INDEX        idx_nakes_faskes   ON nakes(faskes_id);

CREATE TABLE patients (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    faskes_id         UUID          NOT NULL REFERENCES faskes(id) ON DELETE RESTRICT,
    assigned_nakes_id UUID          NOT NULL REFERENCES nakes(id)  ON DELETE RESTRICT,
    username          TEXT          NOT NULL,
    password_hash     TEXT          NOT NULL,
    full_name         TEXT          NOT NULL,
    nik               TEXT          NOT NULL,
    alamat       TEXT          NOT NULL,
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

CREATE TABLE patient_clinical_baselines (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    patient_id          UUID NOT NULL REFERENCES patients(id) ON DELETE CASCADE,
    recorded_at         DATE NOT NULL DEFAULT CURRENT_DATE,
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

-- Index untuk mempercepat query saat menampilkan tren grafik atau load data di dashboard nakes
CREATE INDEX idx_patient_baselines_patient ON patient_clinical_baselines(patient_id);
CREATE INDEX idx_patient_baselines_date    ON patient_clinical_baselines(recorded_at);

COMMIT;
