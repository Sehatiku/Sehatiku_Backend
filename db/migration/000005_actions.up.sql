-- 000005_actions.up.sql
-- Cluster 4: Aksi & Komunikasi — escalations, notifications
-- notifications mendukung penerima pasien, pendamping, MAUPUN nakes
-- (blast kredensial dikirim ke pasien+pendamping saat pasien didaftarkan,
--  dan ke nakes saat nakes didaftarkan).

BEGIN;

-- ---------- escalations (peristiwa klinis + feedback nakes) ----------
CREATE TABLE escalations (
    id                UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    patient_id        UUID              NOT NULL REFERENCES patients(id)    ON DELETE RESTRICT,
    risk_score_id     UUID              NOT NULL REFERENCES risk_scores(id) ON DELETE RESTRICT,
    faskes_id         UUID              NOT NULL REFERENCES faskes(id)      ON DELETE RESTRICT,
    assigned_nakes_id UUID              NOT NULL REFERENCES nakes(id)       ON DELETE RESTRICT,
    tier              escalation_tier   NOT NULL,
    channel           comm_channel      NOT NULL DEFAULT 'whatsapp',
    status            escalation_status NOT NULL DEFAULT 'sent',
    sent_at           TIMESTAMPTZ       NOT NULL DEFAULT now(),
    viewed_at         TIMESTAMPTZ,
    acted_at          TIMESTAMPTZ,
    feedback          escalation_feedback,                 -- NULL = belum dinilai (label emas)
    feedback_by       UUID              REFERENCES nakes(id) ON DELETE SET NULL,
    feedback_at       TIMESTAMPTZ,
    created_at        TIMESTAMPTZ       NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ       NOT NULL DEFAULT now()
);
-- Antrean prioritas dashboard faskes
CREATE INDEX idx_escalations_queue ON escalations(faskes_id, status, tier, sent_at DESC);
-- Query label training (hanya yang sudah dinilai nakes)
CREATE INDEX idx_escalations_label ON escalations(feedback) WHERE feedback IS NOT NULL;

-- ---------- notifications (transport WA/SMS keluar) ----------
-- patient_id DAN nakes_id keduanya nullable; tepat salah satu diisi untuk pesan
-- yang ditujukan ke orang (pesan 'system' boleh keduanya NULL).
CREATE TABLE notifications (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    patient_id          UUID                REFERENCES patients(id)    ON DELETE SET NULL,
    nakes_id            UUID                REFERENCES nakes(id)       ON DELETE SET NULL,
    escalation_id       UUID                REFERENCES escalations(id) ON DELETE SET NULL,
    recipient_phone     TEXT                NOT NULL,
    recipient_role      recipient_role      NOT NULL,
    message_type        message_type        NOT NULL,
    channel             comm_channel        NOT NULL DEFAULT 'whatsapp',
    payload             JSONB,
    status              notification_status NOT NULL DEFAULT 'queued',
    provider_message_id TEXT,
    error_reason        TEXT,
    retry_count         INTEGER             NOT NULL DEFAULT 0,
    queued_at           TIMESTAMPTZ         NOT NULL DEFAULT now(),
    sent_at             TIMESTAMPTZ,
    delivered_at        TIMESTAMPTZ,
    created_at          TIMESTAMPTZ         NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ         NOT NULL DEFAULT now(),
    -- penerima nakes harus ber-role 'nakes'; penerima pasien/pendamping tidak ber-role 'nakes'
    CONSTRAINT chk_recipient_target CHECK (
        (recipient_role = 'nakes'  AND nakes_id IS NOT NULL) OR
        (recipient_role IN ('patient', 'companion') AND patient_id IS NOT NULL) OR
        (message_type = 'system')
    )
);
CREATE INDEX idx_notifications_retry   ON notifications(status, retry_count) WHERE status = 'failed';
CREATE INDEX idx_notifications_patient ON notifications(patient_id);
CREATE INDEX idx_notifications_nakes   ON notifications(nakes_id);

COMMIT;
