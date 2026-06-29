-- Add 'consultation_reply' to message_type enum so in-app notifications
-- created when a nakes replies to a consultation can be persisted.
-- IF NOT EXISTS prevents failure on re-run (idempotent).
ALTER TYPE message_type ADD VALUE IF NOT EXISTS 'consultation_reply';
