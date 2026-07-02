-- Tabel token FCM (Firebase Cloud Messaging) untuk push notification native Patient App.
-- Multi-device: satu pasien boleh punya banyak baris aktif sekaligus. `platform` sengaja
-- TEXT + CHECK (bukan enum Postgres) karena hanya untuk observability/debug, tidak dipakai
-- untuk join/agregasi bisnis. UNIQUE(token) + repository Upsert menangani kasus token yang
-- sama pindah kepemilikan pasien (app uninstall lalu install ulang di HP lain).
CREATE TABLE device_push_tokens (
    id          UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    patient_id  UUID NOT NULL REFERENCES patients(id) ON DELETE CASCADE,
    platform    TEXT NOT NULL CHECK (platform IN ('android','ios')),
    token       TEXT NOT NULL,
    is_active   BOOLEAN NOT NULL DEFAULT true,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (token)
);

CREATE INDEX idx_device_push_tokens_patient ON device_push_tokens(patient_id) WHERE is_active = true;
