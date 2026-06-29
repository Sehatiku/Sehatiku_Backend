-- Drop the old thin table (only hba1c, egfr, bmi, systolic_bp, diastolic_bp).
-- No production data yet — safe to drop and recreate.
DROP TABLE IF EXISTS patient_clinical_baselines;

CREATE TABLE patient_clinical_baselines (
    id                      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    patient_id              UUID NOT NULL REFERENCES patients(id),
    recorded_at             TIMESTAMPTZ NOT NULL DEFAULT now(),

    -- Demographics (stored for ML vector self-containment)
    age_years               SMALLINT    NOT NULL,
    sex                     VARCHAR(10) NOT NULL,   -- male | female

    -- Anthropometry
    bmi                     NUMERIC(5,2) NOT NULL,
    bmi_category            VARCHAR(20)  NOT NULL,  -- underweight | normal | overweight | obese
    waist_circumference_cm  NUMERIC(5,1) NOT NULL,
    central_obesity         BOOLEAN      NOT NULL,

    -- Lifestyle
    smoking_status          VARCHAR(20) NOT NULL,   -- never | former | current
    alcohol_use             BOOLEAN     NOT NULL,
    physical_activity       VARCHAR(20) NOT NULL,   -- sedentary | light | moderate | active

    -- Family history
    family_history_diabetes BOOLEAN NOT NULL,
    family_history_cvd      BOOLEAN NOT NULL,

    -- Blood pressure
    systolic_bp_mmhg        SMALLINT    NOT NULL,
    diastolic_bp_mmhg       SMALLINT    NOT NULL,
    hypertension_status     VARCHAR(30) NOT NULL,   -- normal | elevated | stage1 | stage2

    -- Glucose / diabetes
    fasting_glucose_mgdl    NUMERIC(6,1) NOT NULL,
    hba1c_pct               NUMERIC(4,1) NOT NULL,
    diabetes_status         VARCHAR(30)  NOT NULL,  -- none | prediabetes | type2 | controlled | uncontrolled

    -- Lipid panel
    total_cholesterol_mgdl  NUMERIC(6,1) NOT NULL,
    hdl_mgdl                NUMERIC(5,1) NOT NULL,
    ldl_mgdl                NUMERIC(5,1) NOT NULL,
    triglycerides_mgdl      NUMERIC(6,1) NOT NULL,

    -- CVD risk
    cvd_risk_10yr_pct       NUMERIC(5,2) NOT NULL,
    cvd_risk_category       VARCHAR(20)  NOT NULL,  -- low | moderate | high | very_high

    -- Medications
    on_antihypertensive     BOOLEAN NOT NULL,
    on_antidiabetic         BOOLEAN NOT NULL,
    on_statin               BOOLEAN NOT NULL,

    -- Risk target (ML label)
    target_risk             VARCHAR(20) NOT NULL,

    -- Kidney function
    egfr                    NUMERIC(6,1) NOT NULL,
    uacr                    NUMERIC(8,2) NOT NULL,

    -- ML cluster assignment (nullable — may not be available at registration)
    cluster_id              INTEGER,
    diagnosis_cluster       VARCHAR(100),
    clinical_group          VARCHAR(100),

    created_at              TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX idx_patient_clinical_baselines_patient
    ON patient_clinical_baselines(patient_id, recorded_at DESC);
