-- 000001_init_extensions_and_enums.up.sql
-- Sehatiku MVP — extensions & enum types
-- Dijalankan paling awal: semua tabel bergantung pada pgcrypto (gen_random_uuid) dan enum di bawah.

BEGIN;

-- gen_random_uuid() untuk UUID v4 default PK
CREATE EXTENSION IF NOT EXISTS pgcrypto;

-- ---------- ENUM TYPES ----------
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
CREATE TYPE log_source     AS ENUM ('whatsapp', 'sms', 'web');
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
CREATE TYPE notification_status  AS ENUM ('queued', 'sent', 'delivered', 'failed');

-- Governance ML
CREATE TYPE model_type AS ENUM ('nutrition_match', 'cohort_risk');

COMMIT;
