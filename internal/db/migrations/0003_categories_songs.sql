-- Play Music — categorias passam a conter músicas diretamente.
-- (Álbuns/artistas continuam internos para compatibilidade com o scanner,
-- mas o controle de acesso passa a ser por música.)

CREATE TABLE IF NOT EXISTS category_songs (
    category_id TEXT NOT NULL REFERENCES categories(id) ON DELETE CASCADE,
    song_id TEXT NOT NULL REFERENCES songs(id) ON DELETE CASCADE,
    position INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (category_id, song_id)
);

CREATE INDEX IF NOT EXISTS idx_category_songs_song ON category_songs(song_id);
CREATE INDEX IF NOT EXISTS idx_category_songs_category ON category_songs(category_id, position);
