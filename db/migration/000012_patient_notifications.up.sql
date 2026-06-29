-- 000012_patient_notifications.up.sql
-- Refactor: pisahkan inbox in-app pasien dari tabel transport `notifications`.
--
-- `notifications` kembali MURNI menjadi log transport WA/SMS (credential_blast,
-- daily_prompt, escalation, recommendation, system). Inbox yang dibaca Patient App
-- pindah ke tabel khusus `patient_notifications` dengan state baca/belum-baca (read_at).
--
-- Nilai enum `in_app` (comm_channel) dan `consultation_reply` (message_type) DIBIARKAN
-- ADA di Postgres karena ALTER TYPE ... DROP VALUE tidak didukung; keduanya menjadi
-- DEPRECATED/tidak dipakai lagi setelah migrasi ini (lihat docs/erd.md).

BEGIN;

-- ---------- tipe & tabel inbox in-app ----------
CREATE TYPE patient_notification_type AS ENUM ('consultation_reply', 'daily_reminder');

CREATE TABLE patient_notifications (
    id              UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    patient_id      UUID                      NOT NULL REFERENCES patients(id)      ON DELETE CASCADE,
    type            patient_notification_type NOT NULL,
    title           TEXT                      NOT NULL,   -- teks siap-tampil (ramah lansia)
    body            TEXT                      NOT NULL,
    payload         JSONB,                                -- ekstra per-tipe (nakes_name, run_type, ...)
    consultation_id UUID                      REFERENCES consultations(id) ON DELETE SET NULL,
    read_at         TIMESTAMPTZ,                          -- NULL = belum dibaca
    created_at      TIMESTAMPTZ               NOT NULL DEFAULT now()
);

-- Inbox listing terbaru-dulu per pasien
CREATE INDEX idx_patient_notif_inbox  ON patient_notifications(patient_id, created_at DESC);
-- Badge "belum dibaca" (partial index, hanya baris unread)
CREATE INDEX idx_patient_notif_unread ON patient_notifications(patient_id) WHERE read_at IS NULL;

-- ---------- backfill: pindahkan baris in-app lama ke inbox baru ----------
-- Baris in-app lama selalu bertipe consultation_reply (satu-satunya producer in_app
-- sebelum migrasi ini). Payload lama: {consultation_id, nakes_name, nakes_note}.
INSERT INTO patient_notifications (id, patient_id, type, title, body, payload, consultation_id, read_at, created_at)
SELECT
    n.id,
    n.patient_id,
    'consultation_reply'::patient_notification_type,
    'Balasan dari dokter',
    COALESCE(n.payload->>'nakes_note', ''),
    jsonb_build_object('nakes_name', n.payload->>'nakes_name'),
    NULLIF(n.payload->>'consultation_id', '')::uuid,
    NULL,                       -- semua baris hasil backfill dimulai sebagai belum dibaca
    n.created_at
FROM notifications n
WHERE n.channel = 'in_app'
  AND n.patient_id IS NOT NULL;

-- Bersihkan baris in-app dari tabel transport (sekarang murni WA/SMS).
DELETE FROM notifications WHERE channel = 'in_app';

COMMIT;
