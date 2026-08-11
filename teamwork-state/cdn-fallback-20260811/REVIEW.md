# REVIEW.md — cdn-fallback-20260811

## Task: fallback para o CDN Bunny (?nocdn=1) — commit f0f2e60 (branch feat/cdn-fallback)

**Veredicto: PASS** — nenhum problema bloqueante encontrado. A implementação está coerente com a estratégia acordada (retry no cliente via ?nocdn=1 + karaokê no escopo com 302 assinado + probe logado), os checks de auth precedem o desvio do CDN em ambos os handlers, e não identifiquei regressão nos fluxos existentes (seek, next/prev, PWA/sw.js, karaokê órfão 500). Achados abaixo em ordem de prioridade.

---

### Problemas encontrados (nenhum bloqueante)

**MÉDIO-1 — Probe síncrono sob `cdnMu` no hot path pode travar TODAS as streams por até 10s (a cada 2 min)**
- Arquivo: `internal/stream/stream.go:139-151` (`CDNRangeOK` + `cdnMu`) e `internal/stream/stream.go:166` (`http.Client{Timeout: 10s}`); novo call site amplificador em `internal/server/handlers_karaoke.go:77`.
- Justificativa: `CDNRangeOK` executa o probe segurando `cdnMu`, no caminho de TODA request de stream (música e, agora, karaokê). Se o CDN estiver em estado de "buraco negro" (DNS pendurado, SYN dropado, firewall), o `client.Do` espera os 10s do timeout — e todas as requests concorrentes de streaming bloqueiam no mutex por até 10s, uma vez a cada 2 min (probe global, não por path). No cenário atual de produção (TLS inválido) a falha é rápida (handshake), então o impacto presente é zero — mas é risco latente com CDN real saudável configurada, exatamente o caso que este commit pretende ativar.
- Resposta à pergunta do prompt sobre `context.Background`: sim, o request é limitado pelo `Timeout: 10s` do client (não há vazamento ilimitado; shutdown atrasaria no máximo 10s). O problema não é o timeout, é a sincronicidade no lock.
- Sugestão: (a) timeout curto para o probe (2–3s), (b) probe em background (refresh assíncrono, servindo o valor cached enquanto revalida), ou (c) no mínimo `r.Context()`/context com deadline em vez de `context.Background()`. Não bloqueia o merge, mas recomendo tratar antes de ativar o CDN de verdade.

**MÉDIO-2 — Override PM_E2E_BASEURL é footgun quando a porta alvo está DOWN**
- Arquivo: `e2e/playwright.config.js:47` (`webServer.url = BASE`) vs `e2e/start-server.ps1:8` (hardcoded `http://localhost:4533/`).
- Justificativa: com `PM_E2E_BASEURL=http://localhost:4539` e NADA rodando em :4539, o Playwright executa `start-server.ps1`, que sobe o servidor em :4533 e faz health-check em :4533 (exit 0), enquanto o Playwright espera 60s pela :4539 — falha com erro confuso. O override só funciona quando o servidor de validação já está de pé na porta alvo (que foi o uso real, com `reuseExistingServer`). Para quem roda o DEFAULT (sem env): zero mudança de comportamento, `BASE` == `:4533` == valor antigo — não quebra nada.
- Sugestão: fazer `start-server.ps1` honrar `PM_E2E_BASEURL`, ou documentar explicitamente no README/config que o override exige servidor já rodando.

**BAIXO-3 — Teste e2e novo não exercita o caminho real 302→CDN→falha**
- Arquivo: `e2e/tests/player.spec.js:131-133`.
- Justificativa: o `route.abort('failed')` intercepta a PRIMEIRA request (antes de o servidor responder), ou seja, testa "primeira tentativa falha → ladder retenta com ?nocdn=1". A sequência real (server responde 302 → browser segue para a URL assinada da Bunny → CDN morto → error event) foi validada manualmente, mas não é coberta pelo teste — se um futuro refactor quebrar só o 302 (ex.: redirect para URL sem assinatura), o teste continua verde. Determinismo: OK (abort/fulfill determinísticos; seed atual 100% mp3/wav = nativo → ladder executa; se a 1ª música do catálogo fosse não-nativa o teste falharia com erro confuso — risco baixo, seed atual não tem esse caso).
- Sugestão: quando possível, interceptar e abortar o request ao *target* do redirect (URL da CDN) em vez do request inicial; se inviável em e2e, pelo menos um teste de rota Go (httptest) para o 302 do karaokê.

**BAIXO-4 — Formatos não-nativos perderam o único retry do código antigo**
- Arquivo: `web/assets/player.js:109-114`.
- Justificativa: o handler antigo (31ebabd6) tinha 1 retry (`streamUrl(current, true)`) para TODOS os formatos; o novo para na hora para não-nativos. Em blip transitório de rede, o retry antigo podia recuperar a faixa; agora um blip derruba a música sem segunda chance (a URL de transcode é a mesma, então o retry era "replay da mesma request" — trade-off defensável e documentado no comentário, mas é redução real de resiliência nesse caso). Nativos ganharam 2 retries (nocdn + transcode) — saldo geral positivo.

**BAIXO-5 — Volume de log do probe com CDN fora do ar**
- Arquivo: `internal/stream/stream.go:162,169,176`.
- Justificativa: o probe é global (1 por 2 min por processo, NÃO por path — o prompt supõe "por path diferente", mas `cdnChecked`/`cdnRangeOK` são flags únicas): no máximo ~720 WARN/dia com CDN permanentemente quebrado. Baixo, mas com a config de produção atual (TLS inválido) será ruído permanente em cada deploy. Sugestão: rate-limit (1 WARN após N falhas consecutivas) ou rebaixar para Debug após a primeira.

**BAIXO-6 — Race teórica de `error` atrasado vs. novo load (src re-set redundante)**
- Arquivo: `web/assets/player.js:103-132` e `web/assets/karaoke.js:548-561`.
- Justificativa: se o evento `error` do recurso antigo for despachado depois de `loadAndPlay`/`loadKaraoke` já ter trocado o src (mesmo tick em que o usuário clica next/prev durante uma falha), o handler usa `state.current` (música/karaokê NOVOS) e re-set o src para a variante ?nocdn=1 — degrada o novo item para proxy local (funciona, mas sem CDN). Na prática o load algorithm dispara 'abort' (não 'error') na troca de src; risco muito raro e inofensivo (não quebra playback). Anotação: `attempt`/`videoAttempt` são resets corretos em `loadAndPlay`/`loadKaraoke` — o único ponto é a ausência de guarda por identidade da faixa.

**COSMÉTICO-7 — mp3-nativo no attempt 2 não transcoda (re-cai no branch CDN/local)**
- Arquivo: `web/assets/player.js:123-127` + `internal/server/handlers_media.go:60-70`.
- Justificativa: `streamUrl(song, true)` → `format=mp3`; se `song.format == "mp3"`, o servidor NÃO entra no branch de transcode (`format != song.Format` é falso) e cai no branch nativo → CDN/proxy local. Só ocorre em dupla falha (CDN E proxy local mortos), então na prática é inofensivo; quirk pré-existente do fallback de transcode, inalterado por este commit. Vale um comentário no código.

**COSMÉTICO-8 — Drifts menores**
- `e2e/tests/helpers.js:2` — comentário "must be running on :4533" desatualizado após o PM_E2E_BASEURL.
- `README.md:85` — cita `GET /api/lyrics/{id}` (rota removida no commit d1c9f9b8); drift pré-existente, apenas herdado na edição.
- TTL do link assinado de karaokê = `ND_CDN_TOKENTTL` global (24h default) — paridade com músicas; se karaokê for conteúdo premium, avaliar TTL menor (config global, não por tipo).

---

### Respostas às perguntas específicas

1. **Ordem dos checks (segurança do ?nocdn=1)**: confirmada — `requireAuth` (server.go:113-114, JWT por header ou ?jwt=) → `CanAccessSong`/`CanAccessKaraoke` (handlers_media.go:42-52, handlers_karaoke.go:61-71) → `GetSong`/`GetKaraoke` → format → só então o branch `nocdn`/CDN. `?nocdn=1` NUNCA contorna auth; ele apenas escolhe entre "302 pro CDN" e "ServeNative/ServeVideo local". Sem bypass. O JWT nunca é encaminhado ao CDN (o Location do 302 contém só token+expires da Bunny).
2. **Teste e2e determinístico?** Sim — abort/fulfill determinísticos, `nocdnRequests >= 1` sem race (o request nocdn precede o estado 'Pausar'), e o ignore de `ERR_FAILED` é justificável: só o request abortado produz esse texto, e a asserção de resultado (aria-label + contador) independe do ignore. Não mascara erros reais do fallback — se o ?nocdn=1 falhasse, o aria-label ficaria 'Tocar' e o teste falharia. Única ressalva: o ignore se aplica também a `pageerror` com a mesma substring (teórico) e o abort cobre qualquer request stream não-nocdn (não só o 1º) — cobertura de cenário real coberta no BAIXO-3.
3. **Riscos de produção**: TLS inválido do CDN real → probe falha rápido → fallback local por detecção (comportamento correto, mesmo do binário anterior; este commit só adiciona os WARNs e o 2º call site do probe). Volume de log: BAIXO-5. `context.Background` + Timeout 10s: aceitável, ver MÉDIO-1 para o risco real (lock síncrono no hot path).
4. **Karaokê 302 seguro?** Sim — mesmo modelo de capacidade das músicas: URL assinada (HS256 com `CDNTokenKey`, expires = now+TTL), auth+perm antes do redirect, path `uploads/...` vindo do DB (sem input do usuário), sem JWT no Location. Cliente com o link assinado acessa o objeto pelo TTL — paridade exata com o fluxo de música pré-existente.
5. **PM_E2E_BASEURL quebra o default?** Não — sem a env, `BASE` == `http://localhost:4533`, idêntico ao anterior em config, helpers e webServer. O único risco é o MÉDIO-2 (override com porta alvo down).

---

### Sugestões (não bloqueantes)
- MÉDIO-1: probe com timeout curto/background antes de ativar o CDN real.
- BAIXO-3: teste de rota Go (httptest) para o 302 do karaokê (auth ok → 302 Location assinado; `?nocdn=1` → 200 video/mp4 local).
- BAIXO-4: considerar 1 retry também para não-nativos (replay da mesma URL) ou aceitar o trade-off e documentar no CHANGELOG.
- Cosmético: atualizar comentário helpers.js:2 e remover a menção a /api/lyrics do README.
