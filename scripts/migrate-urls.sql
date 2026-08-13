-- ============================================================================
-- migrate-urls.sql — Reescrita de URLs legadas (CDN / storage) — Play Music
-- ============================================================================
-- Objetivo: zerar referências a hostnames antigos que tenham sobrado no banco.
--
--   Seção 1: MinIO legado  (minios3.centralcursoss.com.br)  → Storage Bunny
--            (jh-s3.storage.bunnycdn.com — Storage Zone "files3", origem S3
--            da pull zone; seguindo o mapeamento aprovado).
--   Seção 2: Pull zone antiga (music.centralcursoss.com.br) → Pull zone nova
--            (files.nuvexaia.online).
--   Seção 3: Storage (jh-s3.storage.bunnycdn.com / bucket files3) — INTOCADO.
--
-- Observações:
--   * O catálogo atual guarda chaves RELATIVAS em songs.path / karaokes.path
--     (ex.: "uploads/<id>.mp3") — nenhum hostname. Os UPDATEs abaixo são um
--     no-op seguro (o WHERE ... LIKE nunca casa) e servem de rede de
--     segurança para linhas legadas que ainda contenham URL completa.
--   * categories.checkout_url NÃO é reescrito de propósito: são links de
--     checkout de pagamento (loja externa), não URLs de mídia/CDN.
--   * Idempotente: replace() de prefixo + WHERE por hostname — rodar de novo
--     não altera nada (segunda passada não encontra mais o host antigo).
--
-- Como rodar (transação única — rollback fácil: ROLLBACK; até o COMMIT):
--   psql "$DATABASE_URL" -f scripts/migrate-urls.sql
-- ou cole o bloco inteiro num psql/pgAdmin (o script já abre BEGIN).
-- ============================================================================

BEGIN;

-- ---------------------------------------------------------------------------
-- Seção 1 — Origem legada MinIO → Storage Bunny (jh-s3.storage.bunnycdn.com)
-- ---------------------------------------------------------------------------
UPDATE songs       SET path      = replace(path,      'minios3.centralcursoss.com.br', 'jh-s3.storage.bunnycdn.com') WHERE path      LIKE '%minios3.centralcursoss.com.br%';
UPDATE karaokes    SET path      = replace(path,      'minios3.centralcursoss.com.br', 'jh-s3.storage.bunnycdn.com') WHERE path      LIKE '%minios3.centralcursoss.com.br%';
UPDATE settings    SET value     = replace(value,     'minios3.centralcursoss.com.br', 'jh-s3.storage.bunnycdn.com') WHERE value     LIKE '%minios3.centralcursoss.com.br%';
UPDATE playlists   SET comment   = replace(comment,   'minios3.centralcursoss.com.br', 'jh-s3.storage.bunnycdn.com') WHERE comment   LIKE '%minios3.centralcursoss.com.br%';
UPDATE play_queue  SET data      = replace(data::text, 'minios3.centralcursoss.com.br', 'jh-s3.storage.bunnycdn.com')::jsonb WHERE data::text LIKE '%minios3.centralcursoss.com.br%';

-- ---------------------------------------------------------------------------
-- Seção 2 — Pull zone antiga → Pull zone nova (files.nuvexaia.online)
-- ---------------------------------------------------------------------------
UPDATE songs       SET path      = replace(path,      'music.centralcursoss.com.br', 'files.nuvexaia.online') WHERE path      LIKE '%music.centralcursoss.com.br%';
UPDATE karaokes    SET path      = replace(path,      'music.centralcursoss.com.br', 'files.nuvexaia.online') WHERE path      LIKE '%music.centralcursoss.com.br%';
UPDATE settings    SET value     = replace(value,     'music.centralcursoss.com.br', 'files.nuvexaia.online') WHERE value     LIKE '%music.centralcursoss.com.br%';
UPDATE playlists   SET comment   = replace(comment,   'music.centralcursoss.com.br', 'files.nuvexaia.online') WHERE comment   LIKE '%music.centralcursoss.com.br%';
UPDATE play_queue  SET data      = replace(data::text, 'music.centralcursoss.com.br', 'files.nuvexaia.online')::jsonb WHERE data::text LIKE '%music.centralcursoss.com.br%';

-- ---------------------------------------------------------------------------
-- Seção 3 — Storage INTACTO (nenhum UPDATE)
-- jh-s3.storage.bunnycdn.com / bucket files3 = origem S3 da pull zone.
-- URLs de storage NÃO são públicas (a entrega é via pull zone com token) —
-- nunca reescrever este hostname no banco.
-- ---------------------------------------------------------------------------

-- ---------------------------------------------------------------------------
-- Verificação — rodar ANTES e DEPOIS do bloco acima (em psql, selecione e
-- execute cada SELECT) para provar a migração:
-- ---------------------------------------------------------------------------
-- SELECT 'legadas_minio'      AS checagem, count(*) FROM songs      WHERE path   LIKE '%minios3.centralcursoss.com.br%';
-- SELECT 'legadas_pullzone'   AS checagem, count(*) FROM songs      WHERE path   LIKE '%music.centralcursoss.com.br%';
-- SELECT 'legadas_settings'   AS checagem, count(*) FROM settings   WHERE value  LIKE '%centralcursoss%';
-- SELECT 'legadas_playlists'  AS checagem, count(*) FROM playlists  WHERE comment LIKE '%centralcursoss%';
-- SELECT 'legadas_queue'      AS checagem, count(*) FROM play_queue WHERE data::text LIKE '%centralcursoss%';
-- SELECT 'storage_intacto'    AS checagem, count(*) FROM songs      WHERE path   LIKE '%jh-s3.storage.bunnycdn.com%';

COMMIT;
