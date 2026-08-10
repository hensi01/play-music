-- Play Music — karaokês (vídeos MP4) como entidade separada das músicas.
-- Acesso por categoria, espelhando o modelo de category_songs.

CREATE TABLE IF NOT EXISTS karaokes (
    id TEXT PRIMARY KEY,
    path TEXT NOT NULL UNIQUE,
    title TEXT NOT NULL,
    artist TEXT NOT NULL DEFAULT '',
    duration REAL NOT NULL DEFAULT 0,
    format TEXT NOT NULL DEFAULT 'mp4',
    size BIGINT NOT NULL DEFAULT 0,
    mtime TIMESTAMPTZ,
    play_count BIGINT NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Karaokês atribuídos a uma categoria (controle de acesso por categoria).
CREATE TABLE IF NOT EXISTS category_karaokes (
    category_id TEXT NOT NULL REFERENCES categories(id) ON DELETE CASCADE,
    karaoke_id TEXT NOT NULL REFERENCES karaokes(id) ON DELETE CASCADE,
    position INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (category_id, karaoke_id)
);

CREATE INDEX IF NOT EXISTS idx_karaokes_path ON karaokes(path);
CREATE INDEX IF NOT EXISTS idx_karaokes_created ON karaokes(created_at DESC);
CREATE INDEX IF NOT EXISTS idx_category_karaokes_karaoke ON category_karaokes(karaoke_id);
CREATE INDEX IF NOT EXISTS idx_category_karaokes_category ON category_karaokes(category_id, position);
