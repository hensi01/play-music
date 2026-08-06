# Play Music — Backend + UI

Play Music é um player web de música estilo Spotify, self-hosted, com interface
em português brasileiro, tema escuro, sidebar, player inferior fixo, player em
tela cheia, playlists, curtidas, histórico, busca e letras sincronizadas.

Este repositório contém:

- **Backend em Go** (`main.go` + `internal/`) — API REST, varredura da biblioteca
  (S3/MinIO), stream (CDN Bunny ou URLs presignadas do MinIO), transcodificação
  com ffmpeg, artwork, letras e autenticação JWT.
- **UI vanilla JS** (`web/assets/`) — embutida no binário via `go:embed` e
  servida na raiz do mesmo domínio.

## Requisitos

- Go 1.26+ (build) — ou Docker para rodar o container.
- PostgreSQL (único banco suportado — **sem SQLite**).
- Bucket S3/MinIO com a biblioteca de músicas.
- ffmpeg (para transcodificação de formatos não-nativos).

## Configuração

Toda a configuração vem do ambiente (variáveis com prefixo `ND_` + `DATABASE_URL`),
documentadas no arquivo `.env`:

| Variável | Descrição |
| --- | --- |
| `ND_ADMINUSERNAME` / `ND_ADMINPASSWORD` | Credenciais do administrador (fonte única) |
| `DATABASE_URL` | Conexão PostgreSQL (`postgres://...`) |
| `ND_MUSICFOLDER` | URL da biblioteca: `s3://bucket?endpoint=...&accessKey=...&secretKey=...&secure=...` (fallback: `ND_S3_*` / `MINIO_*`) |
| `ND_CDN_ENABLED`, `ND_CDN_BASEURL`, `ND_CDN_TOKENAUTHKEY`, `ND_CDN_TOKENTTL`, `ND_CDN_ADVANCEDAUTH`, `ND_CDN_PATH_PREFIX` | Pull Zone Bunny CDN (token Basic MD5 ou Advanced HS256) |
| `ND_REDIS_ENABLED`, `ND_REDIS_URL` | Cache opcional de artwork (fallback: disco) |
| `ND_SCANNER_SCHEDULE` | Agendamento da varredura (ex.: `@every 1h`) |
| `ND_FFMPEGPATH` | Caminho do ffmpeg (opcional se estiver no PATH) |
| `ND_PORT`, `ND_ADDRESS`, `ND_LOGLEVEL` | HTTP server |
| `ND_TRANSCODINGCACHESIZE`, `ND_IMAGECACHESIZE` | Limites de cache em disco |

## Como rodar

```bash
# Local
go build -o play-music .
./play-music          # (variáveis do ambiente carregadas, ex.: dotenv)

# Docker
docker compose up -d --build
```

No primeiro boot as migrations criam o schema no Postgres e a varredura inicial
indexa o bucket (músicas + capas + playlists `.m3u`). O servidor escuta em
`ND_ADDRESS:ND_PORT` (padrão `0.0.0.0:4533`), servindo a UI e a API juntas.

## API

Autenticação: `POST /auth/login` (ou `/auth/createAdmin`) com
`{username, password}` → `{token, id, username, name, isAdmin}`. O JWT viaja no
header `X-ND-Authorization: Bearer <token>` (ou `?jwt=` em URLs de `<img>`/`<audio>`);
o header de resposta pode trazer um token renovado.

Rotas principais (todas exigem JWT, exceto `/auth/*`):

- `GET /api/me`, `GET /api/settings`, `GET /api/home`, `GET /api/search?q=`
- `GET /api/albums`, `GET /api/albums/{id}`, `GET /api/artists`, `GET /api/artists/{id}`, `GET /api/songs/{id}`
- `GET|POST /api/playlists`, `GET|PUT|DELETE /api/playlists/{id}`,
  `POST /api/playlists/{id}/tracks`, `DELETE /api/playlists/{id}/tracks/{entryId}`,
  `PUT /api/playlists/{id}/tracks` (reordenação `{from, to}`)
- `GET /api/me/liked`, `PUT|DELETE /api/me/liked/{id}`
- `GET /api/me/history`, `POST /api/me/history/{id}` (registra reprodução)
- `GET|PUT /api/queue`
- `GET /api/lyrics/{id}`
- `GET /api/artwork/{id}?size=N`
- `GET /api/stream/{id}?format=mp3` (redirect CDN/MinIO ou transcode)
- `POST /api/scan` (varredura manual, opcional)

## Estrutura

```
main.go                 (inicialização, migrations, cron, HTTP server)
internal/config/        (leitura do .env)
internal/db/            (pgxpool + migrations SQL embutidas)
internal/store/         (repositórios PostgreSQL)
internal/storage/       (cliente MinIO/S3)
internal/scanner/       (varredura do bucket: áudio, capas, .m3u)
internal/metadata/      (tags via dhowden/tag + ffprobe)
internal/stream/        (URLs assinadas Bunny CDN / presigned MinIO + transcode)
internal/artwork/       (capas: resize + cache disco/Redis)
internal/lyrics/        (letras embutidas + .lrc sincronizadas)
internal/auth/          (JWT HS256, credenciais do env)
internal/server/        (mux, middlewares CORS/auth, handlers, static)
web/embed.go            (UI embutida)
web/assets/             (front end vanilla JS)
```

## Observação

Para servir a UI junto com a API no mesmo domínio (sem `baseURL`), basta hospedar
os arquivos de `web/assets/` na raiz de um domínio/rota que faça proxy para o
backend (`/api`, `/auth`, `/rest`, etc.).
