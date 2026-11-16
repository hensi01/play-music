CREATE TABLE download_history (
 id BIGSERIAL PRIMARY KEY,
 user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
 song_id TEXT REFERENCES songs(id) ON DELETE SET NULL,
 title TEXT NOT NULL,
 artist TEXT NOT NULL DEFAULT '',
 downloaded_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_download_history_user_date ON download_history(user_id, downloaded_at DESC, id DESC);
