# REVIEW — cdn-hostname-swap (2026-08-13)

Parecer do subagente reviewer sobre a branch `feat/cdn-hostname-swap`
(worktree `play-music-cdn-swap`, base `master` 4b35ea20).

## Escopo

Migração de hostname da pull zone: `music.centralcursoss.com.br` →
`files.nuvexaia.online`. 3 arquivos:

- `web/assets/index.html:22` — comentário de exemplo do `baseURL`
- `.env.example:39-42` — `ND_CDN_BASEURL=https://files.nuvexaia.online`
- `scripts/migrate-urls.sql` (novo) — SQL idempotente, BEGIN/COMMIT

## Veredicto por item

| Item | Veredicto |
|---|---|
| A) Correção do SQL (sintaxe, JSONB, idempotência, storage protegido) | **PASS** |
| B) .env.example consistente com o .env real (storage mantido como endpoint S3) | **PASS** |
| C) index.html — comentário exemplificativo, sem hardcode em Go (cfg.CDNBaseURL) | **PASS** |
| D) Segurança — zero segredos no diff; .env gitignored | **PASS** |

**Final: PASS (4/4)**

## Notas não bloqueantes

1. `index.html:22` — o exemplo `https://files.nuvexaia.online` é a pull zone de
   mídia; semanticamente não é uma API base. Mantido conforme escopo aprovado
   (purgar o host antigo); um exemplo neutro seria mais didático.
2. `STATE.md:47` — prefixos parciais de chave de token CDN (`ec090a62`,
   `50df94e8`) documentados em arquivo commitado (pré-existente, fora do
   escopo) — higiene recomendada: redigir.
3. Migration 0009 (karaokes) está trackeada (commit 454606c1) — `karaokes.path`
   existe e o SQL a cobre corretamente.

## Evidências de validação (execução)

- `go build ./... && go vet ./... && go test ./...` — verdes
  (phone 0.8s / server 2.9s / stream 1.2s)
- Grep: `music.centralcursoss` restante apenas em `STATE.md` (histórico
  aprovado) e no próprio `scripts/migrate-urls.sql` (necessário ao `replace`)
- Live (pull zone nova): song 302 → `files.nuvexaia.online` assinado HS256 →
  **206** audio/wav + `Access-Control-Allow-Origin: *`; sem token → **403**
- Live karaokê: 302 → assinado → **206** video/mp4 + CORS
- e2e smoke (login.spec + player.spec): **11/11 desktop + 11/11 mobile**
