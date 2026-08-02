# Deploy no Coolify

Guia de publicação do **Play Music** (player web estilo Spotify) no Coolify, usando os
serviços MinIO, Redis e Postgres já existentes no servidor.

---

## 1. Pré-requisitos no Coolify

- Servidor Coolify ativo (com Docker)
- **MinIO** rodando no servidor (bucket `play-music`)
- **Redis** rodando no servidor
- **Postgres** (opcional — sem `DATABASE_URL` o app usa SQLite em `/data`)
- Domínio da UI: ex. `musicasplay.centralcursoss.com.br`
- Domínio do Bunny CDN: `music.centralcursoss.com.br` (Pull Zone → MinIO)

## 2. Criar o aplicativo no Coolify

1. **Applications → + Add** → escolha o repositório Git (`hensi01/play-music`).
2. Build Pack: **Dockerfile** (o repositório já possui `Dockerfile` multi-stage
   que compila a UI `web/`, o binário Go e inclui ffmpeg, sqlite e libwebp).
3. **Ports Exposes**: `4533`.
4. **Domains**: `musicasplay.centralcursoss.com.br`.
5. **Persistent Storage**: montar volume em `/data`.

## 3. Variáveis de ambiente

Veja `deploy/coolify.env` para o template completo. O essencial:

```dotenv
ND_MUSICFOLDER=s3://play-music?endpoint=minios3.centralcursoss.com.br&accessKey=...&secretKey=...&secure=true
ND_S3_ENDPOINT=minios3.centralcursoss.com.br
ND_S3_BUCKET=play-music
ND_S3_SECURE=true
ND_REDIS_ENABLED=true
ND_REDIS_URL=redis://default:...@...:6379/0
ND_CDN_ENABLED=true
ND_CDN_BASEURL=https://music.centralcursoss.com.br
ND_CDN_TOKENAUTHKEY=...
ND_CDN_TOKENTTL=24h
ND_CDN_ADVANCEDAUTH=true
DATABASE_URL=postgres://postgres:...@172.x.x.x:5432/play-music?sslmode=disable
ND_PORT=4533
ND_ADDRESS=0.0.0.0
ND_SCANNER_SCHEDULE=@every 1h
```

> ⚠️ **Segurança**: `ND_CDN_TOKENAUTHKEY`, credenciais S3 e `DATABASE_URL` são segredos.

## 4. Rotas do app

- `GET /` → redireciona para `/app/` (UI).
- `POST /auth/login` → login (retorna JWT).
- `GET /api/...` → API REST (me, home, search, albums, artists, playlists,
  liked, history, queue, stream, artwork, lyrics).
- `GET /api/stream/{id}` → 307 para URL assinada do Bunny (formato nativo) ou
  proxy ffmpeg (transcode).

## 5. Verificação pós-deploy

1. Abra a UI, crie o admin no primeiro acesso e entre.
2. Confira se o scan importou as músicas do MinIO (startup + `@every 1h`).
3. Toque uma música: nativa → 307 para o CDN; senão ffmpeg.
4. Verifique os logs: `CDN: redirecting stream to Bunny CDN`.

## 6. Solução de problemas

| Sintoma | Causa provável | Ação |
|---------|----------------|------|
| Scan importa 0 músicas | Credenciais/endpoint do MinIO errados | Confira `ND_S3_*` e o bucket |
| `Redis not reachable` | Redis fora do ar | O app continua em modo memória |
| 403 no CDN | Chave de token auth errada ou algoritmo errado | Confira `ND_CDN_TOKENAUTHKEY` e `ND_CDN_ADVANCEDAUTH` |
| Transcoding falha | ffmpeg ausente | A imagem já inclui ffmpeg |
