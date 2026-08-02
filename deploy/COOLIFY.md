# Deploy no Coolify

Este guia documenta como publicar o Navidrome modificado (MinIO + Redis + Bunny CDN) no
Coolify, usando os serviços MinIO e Redis já existentes no servidor.

---

## 1. Pré-requisitos no Coolify

- Servidor Coolify ativo (com Docker)
- **MinIO** já rodando no servidor (ou em rede acessível)
- **Redis** já rodando no servidor (ou em rede acessível)
- Domínio para a UI: ex. `play.centralcursoss.com.br`
- Domínio do Bunny CDN: `music.centralcursoss.com.br` (Pull Zone → MinIO)

## 2. Criar o aplicativo no Coolify

1. **Applications → + Add** → escolha o repositório Git (este fork do Navidrome).
2. Build Pack: **Dockerfile** (o repositório já possui um `Dockerfile` multi-stage
   que inclui ffmpeg, sqlite, libwebp e mpv).
3. **Ports Exposes**: `4533`.
4. **Domains**: `play.centralcursoss.com.br`.
5. **Persistent Storage**:
   - Montar volume em `/data` (banco SQLite, cache, backups).

## 3. Variáveis de ambiente

Configure as variáveis abaixo no Coolify (aba **Environment Variables**). Use os
valores reais de MinIO/Redis/Bunny.

```dotenv
# --- Idioma / Interface ---
ND_DEFAULTLANGUAGE=pt-BR
ND_DEFAULTTHEME=Dark

# --- Biblioteca no MinIO (S3) ---
ND_MUSICFOLDER=s3://navidrome-music?endpoint=minio.centralcursoss.com.br:9000&accessKey=SEU_ACCESS_KEY&secretKey=SEU_SECRET_KEY&secure=false

# --- MinIO (backend S3) ---
ND_S3_ENDPOINT=minio.centralcursoss.com.br:9000
ND_S3_ACCESSKEY=SEU_ACCESS_KEY
ND_S3_SECRETKEY=SEU_SECRET_KEY
ND_S3_BUCKET=navidrome-music
ND_S3_REGION=us-east-1
ND_S3_SECURE=false

# --- Redis ---
ND_REDIS_ENABLED=true
ND_REDIS_URL=redis://redis.centralcursoss.com.br:6379/0
# ND_REDIS_PASSWORD=se_necessario

# --- Bunny CDN (Pull Zone -> MinIO, S3 origin) ---
ND_CDN_ENABLED=true
ND_CDN_BASEURL=https://music.centralcursoss.com.br
ND_CDN_TOKENAUTHKEY=SUA_CHAVE_TOKEN_AUTH_DO_BUNNY
ND_CDN_TOKENTTL=24h
# Prefixo de path no CDN. Deixe vazio se a Pull Zone aponta direto para a
# biblioteca; preencha (ex: "music") se aponta para a raiz do bucket.
ND_CDN_PATH_PREFIX=

# --- ffmpeg / transcoding ---
ND_TRANSCODINGCACHESIZE=500MiB
ND_TRANSCODING_MAXCONCURRENT=2

# --- Diversos ---
ND_LOGLEVEL=info
ND_ENABLEINSIGHTSCOLLECTOR=false
```

> ⚠️ **Segurança**: `ND_CDN_TOKENAUTHKEY`, `ND_S3_ACCESSKEY` e `ND_S3_SECRETKEY`
> são segredos. Use os **secret resources** do Coolify ou o gerenciador de env
> do Coolify.

## 4. Configurar o Bunny CDN (uma vez)

1. No painel [bunny.net](https://bunny.net), crie uma **Pull Zone**.
2. **Origin URL / S3**: aponte para o MinIO (bucket `navidrome-music`). Use o
   formato S3-compatible do MinIO: `https://minio.centralcursoss.com.br/navidrome-music`
   (ou apenas o bucket se o MinIO expõe o endpoint raiz).
3. **Hostname**: `music.centralcursoss.com.br` (crie o CNAME no DNS apontando
   para o hostname `.b-cdn.net` da Pull Zone).
4. **Security → Token Authentication**: ative e copie a chave para
   `ND_CDN_TOKENAUTHKEY`.
5. **Cache**: configure TTL longo para `audio/*` e `application/ogg`. O Navidrome
   gera URLs assinadas com `expires`, então o CDN pode cachear sem problema.
6. Ative suporte a **Range requests** (padrão) para permitir seek.

**Fluxo resultante**:
```
Cliente → play.centralcursoss.com.br (Navidrome API/UI)
              │  stream/download → 307 para URL assinada
              ▼
        music.centralcursoss.com.br (Bunny CDN)
              ▼
        MinIO (navidrome-music) — S3 origin
```
Formatos nativos (mp3, m4a, ogg, flac, wav) são servidos direto do MinIO pelo
Bunny. Formatos que precisam de transcoding são processados pelo ffmpeg do
Navidrome (lendo do MinIO via pipe) e servidos normalmente.

## 5. Verificação pós-deploy

1. Abra a UI e faça o primeiro login (cria o admin).
2. Verifique se as músicas foram importadas do MinIO (scan automático no startup).
3. Toque uma música: o player deve seguir para o CDN.
4. Teste um formato que exige transcode (ex: forçar opus) e confira nos logs:
   `CDN: redirecting stream to Bunny CDN` (nativos) ou transcodificação normal.

## 6. Solução de problemas

| Sintoma | Causa provável | Ação |
|---------|----------------|------|
| `schema 's3' not registered` | Binário antigo sem o backend S3 | Rebuild da imagem |
| Scan importa 0 músicas | Credenciais/endpoint do MinIO errados | Confira `ND_S3_*` e o bucket |
| `Redis not reachable` | Redis fora do ar | O Navidrome continua em modo memória |
| 403 no CDN | Chave de token auth errada ou token expirado | Confira `ND_CDN_TOKENAUTHKEY` |
| Transcoding falha | ffmpeg ausente no container | A imagem oficial já inclui ffmpeg |
