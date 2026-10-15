-- Migration 025 DOWN: Notification Service tables
-- Rollback for 025_notifications.sql. Drops the trigger first, then the
-- tables (indexes die with their tables). update_updated_at() is owned by
-- 001_extensions_and_enums.sql and is NOT dropped here.

DROP TRIGGER IF EXISTS trg_notification_settings_updated_at ON notification_settings;
DROP TABLE IF EXISTS notification_settings;
DROP TABLE IF EXISTS notifications;
