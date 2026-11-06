-- Qualified listening is private to the account and idempotent per playback.
CREATE TABLE IF NOT EXISTS qualified_listens (
  user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  song_id TEXT NOT NULL REFERENCES songs(id) ON DELETE CASCADE,
  play_id TEXT NOT NULL,
  listened_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (user_id, play_id)
);
CREATE INDEX IF NOT EXISTS idx_qualified_listens_user_time ON qualified_listens(user_id, listened_at DESC);
