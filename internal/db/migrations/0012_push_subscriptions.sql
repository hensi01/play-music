CREATE TABLE IF NOT EXISTS push_subscriptions (
 endpoint TEXT PRIMARY KEY,
 user_id TEXT REFERENCES users(id) ON DELETE SET NULL,
 relay_id TEXT NOT NULL UNIQUE,
 management_token TEXT NOT NULL,
 updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
