-- Migration 009: idempotency key per penerima
-- psql -d praktikum_backend -f migrations/009_notifications_per_recipient.sql
--
-- Satu event course.quota_changed menghasilkan satu notifikasi untuk SETIAP
-- peserta kelas. Karena itu UNIQUE (outbox_id) tidak lagi benar: ia hanya
-- mengizinkan satu penerima per event.
--
-- Kunci penggantinya adalah pasangan (outbox_id, user_id). Pasangan ini
-- tetap idempoten untuk event KRS (satu penerima) maupun untuk perubahan
-- kuota (banyak penerima): task duplikat untuk orang yang sama tidak akan
-- mengirim email kedua, tetapi penerima lain tetap dilayani.

ALTER TABLE notifications
    DROP CONSTRAINT IF EXISTS notifications_outbox_unique;

CREATE UNIQUE INDEX IF NOT EXISTS notifications_outbox_user_unique
    ON notifications (outbox_id, user_id);
