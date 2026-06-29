DROP TABLE IF EXISTS patient_clinical_baselines;

CREATE TABLE patient_clinical_baselines (
    id           UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    patient_id   UUID NOT NULL REFERENCES patients(id),
    recorded_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    hba1c        NUMERIC(4,1),
    egfr         NUMERIC(6,1),
    bmi          NUMERIC(5,2),
    systolic_bp  SMALLINT,
    diastolic_bp SMALLINT,
    created_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);
