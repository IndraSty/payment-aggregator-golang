DROP RULE IF EXISTS no_update_audit_logs ON transaction_audit_logs;
DROP RULE IF EXISTS no_delete_audit_logs ON transaction_audit_logs;
DROP TABLE IF EXISTS transaction_audit_logs;