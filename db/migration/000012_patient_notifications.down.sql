-- 000012_patient_notifications.down.sql
-- Reverse: kembalikan baris consultation_reply ke tabel transport `notifications`
-- (sebagai channel=in_app, seperti sebelum refactor), lalu hapus tabel/tipe inbox.
--
-- CATATAN: baris bertipe `daily_reminder` TIDAK bisa dipulihkan ke notifications
-- (tidak ada padanannya di skema lama) dan akan hilang saat down. Ini operasi
-- rollback/dev, jadi kehilangan data inbox daily_reminder diterima secara sadar.

BEGIN;

INSERT INTO notifications
    (id, patient_id, recipient_phone, recipient_role, message_type, channel,
     payload, status, retry_count, queued_at, sent_at, created_at, updated_at)
SELECT
    pn.id,
    pn.patient_id,
    '',
    'patient'::recipient_role,
    'consultation_reply'::message_type,
    'in_app'::comm_channel,
    jsonb_build_object(
        'consultation_id', pn.consultation_id,
        'nakes_name',      pn.payload->>'nakes_name',
        'nakes_note',      pn.body
    ),
    'sent'::notification_status,
    0,
    pn.created_at,
    pn.created_at,
    pn.created_at,
    pn.created_at
FROM patient_notifications pn
WHERE pn.type = 'consultation_reply';

DROP TABLE patient_notifications;
DROP TYPE patient_notification_type;

COMMIT;
