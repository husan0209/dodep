-- Drop trigger and function
DROP TRIGGER IF EXISTS trigger_payments_updated_at ON payments;
DROP FUNCTION IF EXISTS update_payments_updated_at();

-- Drop table
DROP TABLE IF EXISTS payments;

-- Drop enum
DROP TYPE IF EXISTS payment_status;
