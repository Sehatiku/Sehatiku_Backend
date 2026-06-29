-- Add 'in_app' to comm_channel enum so notifications can target the patient mobile inbox.
-- IF NOT EXISTS prevents failure on re-run (idempotent).
ALTER TYPE comm_channel ADD VALUE IF NOT EXISTS 'in_app';
