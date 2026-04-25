DROP TRIGGER IF EXISTS trg_transactions_updated_at ON transactions;
DROP FUNCTION IF EXISTS update_updated_at_column();
DROP TABLE IF EXISTS transactions;
DROP TYPE IF EXISTS payment_provider;
DROP TYPE IF EXISTS transaction_status;