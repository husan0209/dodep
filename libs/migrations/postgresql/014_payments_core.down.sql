-- Migration 014 DOWN: Payments Core
DROP TRIGGER IF EXISTS trigger_payments_updated_at ON payments;
DROP TRIGGER IF EXISTS trigger_withdrawals_updated_at ON withdrawals;
DROP FUNCTION IF EXISTS update_payments_updated_at();
DROP FUNCTION IF EXISTS update_withdrawals_updated_at();
DROP TABLE IF EXISTS withdrawals CASCADE;
DROP TABLE IF EXISTS payments CASCADE;
DROP TYPE IF EXISTS withdrawal_status;
DROP TYPE IF EXISTS payment_status;
