CREATE TABLE IF NOT EXISTS push_subscriptions (
 endpoint TEXT PRIMARY KEY,
 user_id TEXT REFERENCES users(id) ON DELETE SET NULL,
 relay_id TEXT NOT NULL UNIQUE,
 management_token TEXT NOT NULL,
 active BOOLEAN NOT NULL DEFAULT TRUE,
 operation_id TEXT NOT NULL DEFAULT '',
 subscription_data JSONB NOT NULL,
 updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
