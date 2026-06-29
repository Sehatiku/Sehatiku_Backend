-- 000014_daily_features_roll7_columns.down.sql

BEGIN;

ALTER TABLE daily_features
    DROP COLUMN IF EXISTS glucose_mean_roll7,
    DROP COLUMN IF EXISTS glucose_cv_roll7,
    DROP COLUMN IF EXISTS systolic_roll7,
    DROP COLUMN IF EXISTS sodium_roll7,
    DROP COLUMN IF EXISTS sleep_roll7,
    DROP COLUMN IF EXISTS activity_pct_roll7,
    DROP COLUMN IF EXISTS stress_roll7,
    DROP COLUMN IF EXISTS carbs_roll7;

COMMIT;
