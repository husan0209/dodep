-- 001_kyc.down.sql — reverses the ADDITIVE part of 001_kyc.up.sql only.
--
-- Base tables (kyc_records, kyc_documents, kyc_screenings, kyc_audit_log)
-- are NOT dropped here: they are owned by repository.AutoMigrate and hold
-- compliance data with retention requirements. Only the additive Wave2
-- table rolls back.
DROP TABLE IF EXISTS kyc_limit_overrides;
