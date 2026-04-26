-- Drop trigger and function
DROP TRIGGER IF EXISTS trigger_withdrawals_updated_at ON withdrawals;
DROP FUNCTION IF EXISTS update_withdrawals_updated_at();

-- Drop table
DROP TABLE IF EXISTS withdrawals;

-- Drop enum
DROP TYPE IF EXISTS withdrawal_status;
