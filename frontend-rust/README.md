# Frontend Rust/WASM — em desenvolvimento

Este diretório contém a migração gradual do frontend. A interface JavaScript existente continua sendo a versão funcional; este crate ainda não substitui a aplicação nem deve ser publicado como versão final.

Com Rust, o alvo `wasm32-unknown-unknown` e `wasm-pack` instalados:

```sh
wasm-pack build frontend-rust --target web
```

O módulo fornece cliente HTTP autenticado, reprodução HTML Audio, geração de URL de streaming e estado básico da fila. Ainda faltam integração à interface, controle de eventos e recursos como playlists, karaokê, administração e instalação PWA.
