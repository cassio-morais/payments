-- Drop tables in reverse order
DROP TABLE IF EXISTS webhook_deliveries;
DROP TABLE IF EXISTS webhooks;
DROP TABLE IF EXISTS distributed_locks;
DROP TABLE IF EXISTS outbox;
DROP TABLE IF EXISTS idempotency_keys;
DROP TABLE IF EXISTS account_transactions;
DROP TABLE IF EXISTS payment_events;
DROP TABLE IF EXISTS payments;
DROP TABLE IF EXISTS accounts;

-- Drop functions
DROP FUNCTION IF EXISTS update_updated_at_column();

-- Drop extensions
DROP EXTENSION IF EXISTS "uuid-ossp";
