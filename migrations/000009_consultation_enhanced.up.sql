-- rename old complaint column so we can back-fill if needed, then drop after split
ALTER TABLE consultations RENAME COLUMN complaint TO _complaint_old;

-- structured complaint fields (all required on submission)
ALTER TABLE consultations
    ADD COLUMN complaint_since  TEXT NOT NULL DEFAULT '',
    ADD COLUMN complaint_type   TEXT NOT NULL DEFAULT '',
    ADD COLUMN complaint_detail TEXT NOT NULL DEFAULT '';

-- nakes reply fields (nullable until replied)
ALTER TABLE consultations
    ADD COLUMN nakes_note          TEXT,
    ADD COLUMN replied_by_nakes_id UUID REFERENCES nakes(id),
    ADD COLUMN replied_at          TIMESTAMPTZ;

-- mutable table needs updated_at
ALTER TABLE consultations
    ADD COLUMN updated_at TIMESTAMPTZ NOT NULL DEFAULT now();

-- back-fill the three new columns from old single complaint text
UPDATE consultations SET
    complaint_type   = _complaint_old,
    complaint_detail = ''
WHERE _complaint_old IS NOT NULL AND _complaint_old <> '';

-- drop the legacy column
ALTER TABLE consultations DROP COLUMN _complaint_old;

-- index for nakes query: consultations for patients assigned to a specific nakes
CREATE INDEX idx_consultations_nakes ON consultations(patient_id);
