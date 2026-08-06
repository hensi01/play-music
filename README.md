# Play Music — UI

Front end (UI) do Play Music, um player web de música estilo Spotify, self-hosted.
Interface em português brasileiro, tema escuro, com sidebar, player inferior fixo,
player em tela cheia, playlists, curtidas, histórico, busca e letras sincronizadas.

Este repositório contém **somente a UI** (HTML/CSS/JS vanilla, sem dependências externas).
A API fica em um backend separado (serviço Play Music), que a UI chama via `baseURL`.

## Estrutura

```
web/assets/
├── index.html   (app shell + configuração)
├── style.css
├── app.js       (UI, roteador hash, telas)
├── api.js       (cliente HTTP com JWT + baseURL configurável)
└── player.js    (player de áudio)
```

## Configuração da API

Em `web/assets/index.html`, ajuste `window.__APP_CONFIG__.baseURL`:

```js
window.__APP_CONFIG__ = {
  version: 'dev',
  firstTime: false,
  // URL base da API do backend. Deixe vazio para o mesmo domínio do front end.
  baseURL: ''
}
```

- `baseURL` vazio (`''`) — a UI chama `/api/*` e `/auth/*` no mesmo domínio onde está hospedada.
- `baseURL` preenchido (ex.: `https://music.centralcursoss.com.br`) — a UI aponta para um
  backend remoto. Nesse caso o backend precisa aceitar CORS (origem do front end) e o acesso
  via `?jwt=` nos endpoints de stream/artwork.

A autenticação usa JWT: token salvo no `localStorage` e enviado no header
`X-ND-Authorization` (ou como `?jwt=` em URLs de `<img>`/`<audio>`).

## Como servir

Qualquer servidor estático. Exemplos:

```bash
# Python
python -m http.server 8080 -d web/assets

# Node
npx serve web/assets -l 8080

# Docker (nginx)
docker run --rm -p 8080:80 -v "$PWD/web/assets:/usr/share/nginx/html:ro" nginx:alpine
```

Abra `http://localhost:8080` no navegador.

## Observação

Para servir a UI junto com a API no mesmo domínio (sem `baseURL`), basta hospedar os
arquivos de `web/assets/` na raiz de um domínio/rota que faça proxy para o backend
(`/api`, `/auth`, `/rest`, etc.).
