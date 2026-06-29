-- 000014_daily_features_roll7_columns.up.sql
-- daily_features dibuat di 000004 dengan skema lama (glucose_avg, glucose_roll7_mean,
-- activity_minutes, stress_level, ...). Integrasi ML memakai 8 fitur roll-7 yang
-- dinamai sesuai kontrak model (entity.DailyFeature / ml.DailyAverage). Tambahkan
-- kolom yang dibutuhkan agar DailyFeatureRepository bisa menulis & membaca. Kolom lama
-- dibiarkan (nullable, tidak dipakai oleh kode ML saat ini).

BEGIN;

ALTER TABLE daily_features
    ADD COLUMN IF NOT EXISTS glucose_mean_roll7 NUMERIC,
    ADD COLUMN IF NOT EXISTS glucose_cv_roll7   NUMERIC,
    ADD COLUMN IF NOT EXISTS systolic_roll7     NUMERIC,
    ADD COLUMN IF NOT EXISTS sodium_roll7       NUMERIC,
    ADD COLUMN IF NOT EXISTS sleep_roll7        NUMERIC,
    ADD COLUMN IF NOT EXISTS activity_pct_roll7 NUMERIC,
    ADD COLUMN IF NOT EXISTS stress_roll7       NUMERIC,
    ADD COLUMN IF NOT EXISTS carbs_roll7        NUMERIC;

COMMIT;
