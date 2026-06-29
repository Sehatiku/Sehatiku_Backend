DROP INDEX IF EXISTS idx_consultations_nakes;

ALTER TABLE consultations DROP COLUMN IF EXISTS updated_at;
ALTER TABLE consultations DROP COLUMN IF EXISTS replied_at;
ALTER TABLE consultations DROP COLUMN IF EXISTS replied_by_nakes_id;
ALTER TABLE consultations DROP COLUMN IF EXISTS nakes_note;
ALTER TABLE consultations DROP COLUMN IF EXISTS complaint_detail;
ALTER TABLE consultations DROP COLUMN IF EXISTS complaint_type;
ALTER TABLE consultations DROP COLUMN IF EXISTS complaint_since;

ALTER TABLE consultations ADD COLUMN complaint TEXT NOT NULL DEFAULT '';
