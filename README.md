# Play Music — Backend + UI

Play Music é um player web de música estilo Spotify, self-hosted, com interface
em português brasileiro, tema escuro, sidebar, player inferior fixo, player em
tela cheia, playlists, curtidas, histórico, busca e letras sincronizadas.

**Multiusuário por categorias**: o administrador cria categorias (ex.: Cristão,
Rock), atribui álbuns e artistas a elas e libera categorias por cliente. Cada
cliente acessa com o **telefone `(99) 99999-9999` apenas** (sem senha — o acesso
vem da conta criada pelo admin) e só enxerga e reproduz o conteúdo das
categorias liberadas — inclusive `stream`/`artwork` (403 fora do acesso).
Playlists são pessoais e só aceitam músicas liberadas.

Este repositório contém:

- **Backend em Go** (`main.go` + `internal/`) — API REST, varredura da biblioteca
  (S3/MinIO), stream (CDN Bunny ou URLs presignadas do MinIO), transcodificação
  com ffmpeg, artwork (inclui upload de foto por álbum), letras, autenticação
  JWT, usuários e controle de acesso por categoria.
- **UI vanilla JS** (`web/assets/`) — embutida no binário via `go:embed` e
  servida na raiz do mesmo domínio. Inclui página de administração (`#/admin`).

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
| `ND_CDN_ENABLED`, `ND_CDN_BASEURL`, `ND_CDN_TOKENAUTHKEY`, `ND_CDN_TOKENTTL`, `ND_CDN_ADVANCEDAUTH`, `ND_CDN_PATH_PREFIX` | Pull Zone Bunny CDN (token Basic MD5 ou Advanced HS256). Áudio e karaokê usam o CDN quando a zona responde Range (probe); com o CDN fora do ar, o cliente cai automaticamente para o proxy local (`?nocdn=1`) e, em áudio, ainda há o fallback de transcode |
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

Autenticação: `POST /auth/login` com `{username, password}` (administrador) ou
`{phone}` (cliente — **somente o telefone**, aceito com ou sem máscara;
desconhecido → 401)
→ `{token, id, name, phone, isAdmin}`. O JWT viaja no header
`X-ND-Authorization: Bearer <token>` (ou `?jwt=` em URLs de `<img>`/`<audio>`);
o header de resposta pode trazer um token renovado.

O administrador inicial é criado no primeiro boot a partir de
`ND_ADMINUSERNAME`/`ND_ADMINPASSWORD`.

Rotas principais (todas exigem JWT, exceto `/auth/login`):

- `GET /api/me`, `GET /api/settings`, `GET /api/home` (cliente: seções por
  categoria liberada), `GET /api/search?q=`, `GET /api/categories`
- `GET /api/albums`, `GET /api/albums/{id}`, `GET /api/artists`, `GET /api/artists/{id}`, `GET /api/songs/{id}`
- Playlists pessoais: `GET|POST /api/playlists`, `GET|PUT|DELETE /api/playlists/{id}`,
  `POST /api/playlists/{id}/tracks` (só músicas liberadas), `DELETE /api/playlists/{id}/tracks/{entryId}`,
  `PUT /api/playlists/{id}/tracks` (reordenação `{from, to}`)
- `GET /api/me/liked`, `PUT|DELETE /api/me/liked/{id}` (por usuário)
- `GET /api/me/history`, `POST /api/me/history/{id}` (por usuário)
- `GET|PUT /api/queue` (por usuário)
- `GET /api/lyrics/{id}`, `GET /api/artwork/{id}?size=N`, `GET /api/stream/{id}?format=mp3&nocdn=1`
  (`?nocdn=1` força o proxy local, sem redirect para o CDN — fallback usado
  pelo player quando a URL do CDN falha), `GET /api/karaoke/stream/{id}?nocdn=1`

Admin (JWT + `is_admin`):

- `GET|POST /api/admin/users`, `PUT|DELETE /api/admin/users/{id}` (criar/editar
  clientes com telefone, senha opcional e categorias liberadas — o cliente
  entra só com o telefone)
- `GET|POST /api/admin/categories`, `GET|PUT|DELETE /api/admin/categories/{id}`
  (atribuição de álbuns e artistas via `{name, albumIds, artistIds}`)
- `GET /api/admin/albums`, `GET /api/admin/artists`
- `POST /api/admin/albums/{id}/photo` (multipart `photo`, máx 15MB),
  `DELETE /api/admin/albums/{id}/photo` (volta à capa embutida)
- `POST /api/scan` (varredura manual)

## Estrutura

```
main.go                 (inicialização, migrations, cron, HTTP server)
internal/config/        (leitura do .env)
internal/db/            (pgxpool + migrations SQL embutidas)
internal/store/         (repositórios PostgreSQL + controle de acesso)
internal/storage/       (cliente MinIO/S3)
internal/scanner/       (varredura do bucket: áudio, capas, .m3u)
internal/metadata/      (tags via dhowden/tag + ffprobe)
internal/stream/        (URLs assinadas Bunny CDN / presigned MinIO + transcode)
internal/artwork/       (capas: resize + cache disco/Redis + upload de foto)
internal/lyrics/        (letras embutidas + .lrc sincronizadas)
internal/phone/         (normalização/máscara de telefone BR)
internal/auth/          (JWT HS256, bootstrap do admin via env, login telefone/usuário)
internal/server/        (mux, middlewares CORS/auth/admin, handlers, static)
web/embed.go            (UI embutida)
web/assets/             (front end vanilla JS + página de administração)
```

## Observação

Para servir a UI junto com a API no mesmo domínio (sem `baseURL`), basta hospedar
os arquivos de `web/assets/` na raiz de um domínio/rota que faça proxy para o
backend (`/api`, `/auth`, `/rest`, etc.).
