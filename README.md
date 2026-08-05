# Play Music

Player web de música estilo Spotify, self-hosted. Toca **sua** biblioteca de música hospedada
no MinIO/S3, servida via Bunny CDN com URLs assinadas, com transcoding via ffmpeg quando necessário.

Baseado no [Navidrome](https://github.com/navidrome/navidrome), com UI nova (React/Vite/Tailwind)
estilo Spotui/Spotify e API REST própria.

## Recursos

- 🎨 UI estilo Spotify (sidebar, player inferior fixo, player em tela cheia, tema escuro)
- 🗂️ Biblioteca em **MinIO/S3** (buckets, scanner automático)
- 🎵 Streaming via **Bunny CDN** com tokens assinados (HMAC-SHA256 ou MD5)
- 🔄 Transcoding via **ffmpeg** (lê do S3 via pipe) para formatos que o navegador não toca
- 🧠 **Redis** para sessões, now-playing, rate-limit e lock do scanner
- 💾 Postgres ou SQLite (sem `DATABASE_URL` usa SQLite em `/data`)
- ❤️ Curtidas, playlists, histórico, busca, letras sincronizadas (LRC/embutidas)
- 📝 Interface em português brasileiro

## Arquitetura

```
Browser → /app (UI) + /api (REST) + /auth/login (JWT)
               │  stream nativo → 307 para URL assinada
               ▼
         Bunny CDN (Pull Zone → S3 origin)
               ▼
         MinIO — bucket play-music
```

## Rotas da API

| Rota | Função |
|---|---|
| `POST /auth/login` · `POST /auth/createAdmin` | Autenticação (JWT) |
| `GET /api/me` · `GET /api/settings` | Perfil e configurações |
| `GET /api/home` | Seções estilo Spotify (recentes, novos, mais tocados) |
| `GET /api/search?q=` | Busca de músicas, álbuns, artistas e playlists |
| `GET /api/albums` · `/api/albums/{id}` | Álbuns (lista e detalhe com faixas) |
| `GET /api/artists` · `/api/artists/{id}` | Artistas (detalhe com top + álbuns) |
| `GET /api/songs/{id}` | Faixa |
| `CRUD /api/playlists` + tracks | Playlists (criar, adicionar, remover, reordenar) |
| `GET/PUT/DELETE /api/me/liked` | Curtidas |
| `GET /api/me/history` · `POST /api/me/history/{id}` | Histórico e contagem de plays |
| `GET/PUT /api/queue` | Fila persistida |
| `GET /api/stream/{id}` | 307 → CDN (formato nativo) ou ffmpeg (transcode) |
| `GET /api/artwork/{id}` | Capas |
| `GET /api/lyrics/{id}` | Letras sincronizadas |

## Desenvolvimento local

O projeto é 100% Go — a UI (HTML/CSS/JS) fica em `web/assets/` e é embutida no binário via `//go:embed`. Não há Node, npm ou TypeScript.

```bash
set CGO_ENABLED=1
set CC=%USERPROFILE%\scoop\apps\gcc\current\bin\gcc.exe   # Windows com GCC
go build -tags "netgo sqlite_fts5" -o playmusic.exe .
$env:ND_MUSICFOLDER="caminho/para/sua/musica"; $env:ND_DATAFOLDER="tmp/data"; .\playmusic.exe
```

A UI fica disponível em `http://localhost:4533/app/`.

## Deploy no Coolify

Veja [`deploy/COOLIFY.md`](deploy/COOLIFY.md) e [`deploy/coolify.env`](deploy/coolify.env).

## Configuração principal (variáveis `ND_*`)

- `ND_MUSICFOLDER` — URI S3 da biblioteca, ex.
  `s3://play-music?endpoint=minios3.centralcursoss.com.br&accessKey=...&secretKey=...&secure=true`
- `ND_S3_ENDPOINT`, `ND_S3_BUCKET`, `ND_S3_ACCESSKEY`, `ND_S3_SECRETKEY`, `ND_S3_SECURE`
- `ND_REDIS_ENABLED`, `ND_REDIS_URL` — Redis para sessões/estado (opcional)
- `ND_CDN_ENABLED`, `ND_CDN_BASEURL`, `ND_CDN_TOKENAUTHKEY`, `ND_CDN_TOKENTTL`,
  `ND_CDN_ADVANCEDAUTH` — Bunny CDN (opcional)
- `DATABASE_URL` — Postgres (opcional; sem isso usa SQLite em `/data`)
- `ND_PORT`, `ND_ADDRESS`, `ND_LOGLEVEL`, `ND_SCANNER_SCHEDULE`
