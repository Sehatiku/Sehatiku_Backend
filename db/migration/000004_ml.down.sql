-- 000004_ml.down.sql

BEGIN;

DROP TABLE IF EXISTS risk_scores;
DROP TABLE IF EXISTS daily_features;
DROP TABLE IF EXISTS model_versions;

COMMIT;
