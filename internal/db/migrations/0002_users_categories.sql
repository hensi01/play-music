-- Play Music — multiusuário, categorias e permissões por categoria.

CREATE TABLE IF NOT EXISTS users (
    id TEXT PRIMARY KEY,
    username TEXT UNIQUE,
    phone TEXT UNIQUE,
    name TEXT NOT NULL DEFAULT '',
    password_hash TEXT NOT NULL,
    is_admin BOOLEAN NOT NULL DEFAULT false,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS categories (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE UNIQUE INDEX IF NOT EXISTS idx_categories_name ON categories (lower(name));

-- Albums and artists assigned to a category (both grant access).
CREATE TABLE IF NOT EXISTS category_albums (
    category_id TEXT NOT NULL REFERENCES categories(id) ON DELETE CASCADE,
    album_id TEXT NOT NULL REFERENCES albums(id) ON DELETE CASCADE,
    PRIMARY KEY (category_id, album_id)
);
CREATE TABLE IF NOT EXISTS category_artists (
    category_id TEXT NOT NULL REFERENCES categories(id) ON DELETE CASCADE,
    artist_id TEXT NOT NULL REFERENCES artists(id) ON DELETE CASCADE,
    PRIMARY KEY (category_id, artist_id)
);

-- Category grants per user (clients).
CREATE TABLE IF NOT EXISTS user_categories (
    user_id TEXT NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    category_id TEXT NOT NULL REFERENCES categories(id) ON DELETE CASCADE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id, category_id)
);

-- Playlists become personal (owner). Legacy rows are backfilled to the admin
-- by the Go bootstrap.
ALTER TABLE playlists ADD COLUMN IF NOT EXISTS user_id TEXT REFERENCES users(id) ON DELETE CASCADE;

-- Likes become per-user.
ALTER TABLE user_likes DROP CONSTRAINT IF EXISTS user_likes_pkey;
ALTER TABLE user_likes ADD COLUMN IF NOT EXISTS user_id TEXT;
ALTER TABLE user_likes ADD PRIMARY KEY (user_id, entity_type, entity_id);

-- History becomes per-user.
ALTER TABLE history ADD COLUMN IF NOT EXISTS user_id TEXT REFERENCES users(id) ON DELETE CASCADE;

-- Play queue becomes per-user (one row per user). Legacy rows are dropped
-- here (recreated by the Go bootstrap backfill if any).
ALTER TABLE play_queue DROP CONSTRAINT IF EXISTS play_queue_id_check;
ALTER TABLE play_queue DROP CONSTRAINT IF EXISTS play_queue_pkey;
ALTER TABLE play_queue ADD COLUMN IF NOT EXISTS user_id TEXT;
DELETE FROM play_queue WHERE user_id IS NULL;
ALTER TABLE play_queue ADD PRIMARY KEY (user_id);

CREATE INDEX IF NOT EXISTS idx_category_albums_album ON category_albums(album_id);
CREATE INDEX IF NOT EXISTS idx_category_artists_artist ON category_artists(artist_id);
CREATE INDEX IF NOT EXISTS idx_user_categories_user ON user_categories(user_id);
CREATE INDEX IF NOT EXISTS idx_playlists_user ON playlists(user_id);
CREATE INDEX IF NOT EXISTS idx_history_user ON history(user_id);
