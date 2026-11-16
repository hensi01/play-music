use crate::{api_request, get_token, set_token, stream_url, BrowserPlayer, Song};
use std::{cell::RefCell, rc::Rc};
use wasm_bindgen::{closure::Closure, prelude::*, JsCast};
use wasm_bindgen_futures::{spawn_local, JsFuture};
use web_sys::{Document, Element, Event, HtmlInputElement};

struct App {
    document: Document,
    root: Element,
    player: BrowserPlayer,
    songs: Vec<Song>,
    current: Option<usize>,
    queue: Vec<Song>,
    shuffle: bool,
    repeat: bool,
    videos: Vec<Video>,
}

#[derive(Clone, serde::Deserialize)]
struct Video {
    id: String,
    title: String,
    #[serde(default)]
    artist: String,
}
#[derive(Clone, serde::Deserialize)]
struct Playlist {
    id: String,
    name: String,
    #[serde(default)]
    songs: Vec<PlaylistEntry>,
}
#[derive(Clone, serde::Deserialize)]
struct PlaylistEntry {
    #[serde(rename = "entryId")]
    entry_id: String,
    song: Song,
}

fn element(document: &Document, tag: &str, text: &str) -> Result<Element, JsValue> {
    let node = document.create_element(tag)?;
    node.set_text_content(Some(text));
    Ok(node)
}

fn show_error(app: &App, message: &str) {
    if let Some(node) = app.document.get_element_by_id("error") {
        node.set_text_content(Some(message));
    }
}

fn render_login(app: &App) -> Result<(), JsValue> {
    app.root.set_inner_html("");
    app.root
        .append_child(&element(&app.document, "h1", "Play Music")?)?;
    let phone = app.document.create_element("input")?;
    phone.set_attribute("id", "phone")?;
    phone.set_attribute("type", "tel")?;
    phone.set_attribute("placeholder", "Telefone")?;
    app.root.append_child(&phone)?;
    let button = element(&app.document, "button", "Entrar")?;
    button.set_attribute("data-action", "login")?;
    app.root.append_child(&button)?;
    let username = app.document.create_element("input")?;
    username.set_attribute("id", "username")?;
    username.set_attribute("placeholder", "Usuário administrador")?;
    username.set_attribute("autocomplete", "username")?;
    app.root.append_child(&username)?;
    let password = app.document.create_element("input")?;
    password.set_attribute("id", "password")?;
    password.set_attribute("type", "password")?;
    password.set_attribute("placeholder", "Senha")?;
    password.set_attribute("autocomplete", "current-password")?;
    app.root.append_child(&password)?;
    let admin = element(&app.document, "button", "Entrar como administrador")?;
    admin.set_attribute("data-action", "login-admin")?;
    app.root.append_child(&admin)?;
    let error = element(&app.document, "p", "")?;
    error.set_attribute("id", "error")?;
    app.root.append_child(&error)?;
    Ok(())
}

fn render_library(app: &App) -> Result<(), JsValue> {
    app.root.set_inner_html("");
    app.root
        .append_child(&element(&app.document, "h1", "Play Music — Músicas")?)?;
    for (action, label) in [
        ("refresh", "Músicas"),
        ("liked", "Curtidas"),
        ("history", "Histórico"),
        ("playlists", "Playlists"),
        ("karaoke", "Karaokê"),
        ("categories", "Categorias"),
        ("albums", "Álbuns"),
        ("artists", "Artistas"),
        ("previous", "Anterior"),
        ("pause", "Pausar"),
        ("resume", "Continuar"),
        ("next", "Próxima"),
        ("shuffle", "Aleatório"),
        ("repeat", "Repetir"),
        ("logout", "Sair"),
    ] {
        let button = element(&app.document, "button", label)?;
        button.set_attribute("data-action", action)?;
        app.root.append_child(&button)?;
    }
    let search = app.document.create_element("input")?;
    search.set_attribute("id", "search")?;
    search.set_attribute("placeholder", "Buscar músicas")?;
    app.root.append_child(&search)?;
    let search_button = element(&app.document, "button", "Buscar")?;
    search_button.set_attribute("data-action", "search")?;
    app.root.append_child(&search_button)?;
    let error = element(&app.document, "p", "")?;
    error.set_attribute("id", "error")?;
    app.root.append_child(&error)?;
    let list = app.document.create_element("ul")?;
    for (index, song) in app.songs.iter().enumerate() {
        let row = app.document.create_element("li")?;
        let button = element(
            &app.document,
            "button",
            &format!("{} — {}", song.title, song.artist),
        )?;
        button.set_attribute("data-song", &index.to_string())?;
        row.append_child(&button)?;
        let like = element(
            &app.document,
            "button",
            if song.liked { "Descurtir" } else { "Curtir" },
        )?;
        like.set_attribute("data-like", &index.to_string())?;
        row.append_child(&like)?;
        list.append_child(&row)?;
    }
    app.root.append_child(&list)?;
    Ok(())
}

fn media_session() -> Option<JsValue> {
    let window = web_sys::window()?;
    let navigator = js_sys::Reflect::get(window.as_ref(), &"navigator".into()).ok()?;
    let session = js_sys::Reflect::get(&navigator, &"mediaSession".into()).ok()?;
    if session.is_undefined() || session.is_null() {
        None
    } else {
        Some(session)
    }
}

fn set_metadata(song: &Song) {
    let Some(session) = media_session() else {
        return;
    };
    let Some(window) = web_sys::window() else {
        return;
    };
    let Ok(constructor) = js_sys::Reflect::get(window.as_ref(), &"MediaMetadata".into()) else {
        return;
    };
    let Ok(constructor) = constructor.dyn_into::<js_sys::Function>() else {
        return;
    };
    let metadata = js_sys::Object::new();
    let _ = js_sys::Reflect::set(&metadata, &"title".into(), &song.title.clone().into());
    let _ = js_sys::Reflect::set(&metadata, &"artist".into(), &song.artist.clone().into());
    let _ = js_sys::Reflect::set(&metadata, &"album".into(), &song.album.clone().into());
    if let Ok(params) = web_sys::UrlSearchParams::new() {
        params.set("size", "512");
        if let Ok(Some(token)) = get_token() {
            params.set("jwt", &token);
        }
        let artwork = js_sys::Object::new();
        let url = format!(
            "/api/artwork/{}?{}",
            js_sys::encode_uri_component(&song.id),
            String::from(params.to_string())
        );
        let _ = js_sys::Reflect::set(&artwork, &"src".into(), &url.into());
        let _ = js_sys::Reflect::set(&artwork, &"sizes".into(), &"512x512".into());
        let list = js_sys::Array::new();
        list.push(&artwork);
        let _ = js_sys::Reflect::set(&metadata, &"artwork".into(), &list);
    }
    let args = js_sys::Array::new();
    args.push(&metadata);
    if let Ok(value) = js_sys::Reflect::construct(&constructor, &args) {
        let _ = js_sys::Reflect::set(&session, &"metadata".into(), &value);
    }
}

fn sync_media_state(app: &App) {
    let Some(session) = media_session() else {
        return;
    };
    let playing = if app.player.is_paused() {
        "paused"
    } else {
        "playing"
    };
    let _ = js_sys::Reflect::set(&session, &"playbackState".into(), &playing.into());
    let duration = app.player.duration();
    let position = app.player.position();
    if !duration.is_finite() || duration <= 0.0 || !position.is_finite() {
        return;
    }
    let state = js_sys::Object::new();
    let _ = js_sys::Reflect::set(&state, &"duration".into(), &duration.into());
    let _ = js_sys::Reflect::set(
        &state,
        &"position".into(),
        &position.clamp(0.0, duration).into(),
    );
    let _ = js_sys::Reflect::set(&state, &"playbackRate".into(), &1.0.into());
    if let Ok(method) = js_sys::Reflect::get(&session, &"setPositionState".into())
        .and_then(|v| v.dyn_into::<js_sys::Function>())
    {
        let _ = method.call1(&session, &state);
    }
}

fn install_media_session(app: Rc<RefCell<App>>) {
    let Some(session) = media_session() else {
        return;
    };
    let Ok(setter) = js_sys::Reflect::get(&session, &"setActionHandler".into())
        .and_then(|v| v.dyn_into::<js_sys::Function>())
    else {
        return;
    };
    for action in [
        "play",
        "pause",
        "nexttrack",
        "previoustrack",
        "seekto",
        "seekbackward",
        "seekforward",
    ] {
        let state = app.clone();
        let name = action.to_string();
        let callback = Closure::<dyn FnMut(JsValue)>::new(move |details: JsValue| {
            let mut app = state.borrow_mut();
            match name.as_str() {
                "play" => {
                    let _ = app.player.element().play();
                }
                "pause" => {
                    let _ = app.player.pause();
                }
                "nexttrack" => {
                    if let Some(index) = app.current {
                        if let Some(next) = next_index(&app, index) {
                            play_track(&mut app, next);
                        }
                    }
                }
                "previoustrack" => {
                    if let Some(index) = app.current {
                        play_track(&mut app, index.saturating_sub(1));
                    }
                }
                "seekto" => {
                    if let Ok(value) = js_sys::Reflect::get(&details, &"seekTime".into()) {
                        if let Some(position) = value.as_f64() {
                            app.player.seek(position);
                        }
                    }
                }
                "seekbackward" | "seekforward" => {
                    let offset = js_sys::Reflect::get(&details, &"seekOffset".into())
                        .ok()
                        .and_then(|n| n.as_f64())
                        .unwrap_or(10.0);
                    let position = app.player.position()
                        + if name == "seekbackward" {
                            -offset
                        } else {
                            offset
                        };
                    app.player.seek(position);
                }
                _ => {}
            }
        });
        if setter
            .call2(&session, &action.into(), callback.as_ref())
            .is_ok()
        {
            callback.forget();
        }
    }
}

fn next_index(app: &App, current: usize) -> Option<usize> {
    if app.queue.is_empty() {
        return None;
    }
    if app.shuffle && app.queue.len() > 1 {
        let offset = (js_sys::Math::random() * (app.queue.len() - 1) as f64) as usize + 1;
        return Some((current + offset) % app.queue.len());
    }
    if current + 1 < app.queue.len() {
        Some(current + 1)
    } else if app.repeat {
        Some(0)
    } else {
        None
    }
}

fn play_track(app: &mut App, index: usize) {
    if let Some(song) = app.queue.get(index).cloned() {
        if let Ok(url) = stream_url("", &song.id, &song.format) {
            set_metadata(&song);
            app.player.load(&url);
            app.current = Some(index);
            let saved = serde_json::json!({"songs": app.queue, "currentIndex": index, "shuffle": app.shuffle, "repeat": app.repeat}).to_string();
            spawn_local(async move {
                let _ = api_request("/api/queue".into(), "PUT".into(), Some(saved)).await;
            });
            let _ = app.player.element().play();
            spawn_local(async move {
                let _ =
                    api_request(format!("/api/me/history/{}", song.id), "POST".into(), None).await;
            });
        }
    }
}

async fn library(app: Rc<RefCell<App>>) -> Result<(), JsValue> {
    let response = api_request("/api/songs".into(), "GET".into(), None).await?;
    if !response.ok() {
        return Err(JsValue::from_str("Não foi possível carregar as músicas"));
    }
    let data = JsFuture::from(response.json()?).await?;
    let songs: Vec<Song> =
        serde_wasm_bindgen::from_value(data).map_err(|e| JsValue::from_str(&e.to_string()))?;
    let mut app = app.borrow_mut();
    app.songs = songs;
    render_library(&app)
}

async fn songs_from(app: Rc<RefCell<App>>, path: String, search: bool) -> Result<(), JsValue> {
    let response = api_request(path, "GET".into(), None).await?;
    if !response.ok() {
        return Err(JsValue::from_str("Não foi possível carregar as músicas"));
    }
    let data = JsFuture::from(response.json()?).await?;
    let songs = if search {
        js_sys::Reflect::get(&data, &"songs".into())?
    } else {
        data
    };
    let songs: Vec<Song> =
        serde_wasm_bindgen::from_value(songs).map_err(|e| JsValue::from_str(&e.to_string()))?;
    let mut app = app.borrow_mut();
    app.songs = songs;
    render_library(&app)
}

async fn karaoke(app: Rc<RefCell<App>>) -> Result<(), JsValue> {
    let response = api_request("/api/karaokes".into(), "GET".into(), None).await?;
    if !response.ok() {
        return Err(JsValue::from_str("Não foi possível carregar o karaokê"));
    }
    let data = JsFuture::from(response.json()?).await?;
    let videos: Vec<Video> =
        serde_wasm_bindgen::from_value(data).map_err(|e| JsValue::from_str(&e.to_string()))?;
    let mut state = app.borrow_mut();
    state.videos = videos;
    render_library(&state)?;
    let title = element(&state.document, "h2", "Karaokê")?;
    state.root.append_child(&title)?;
    let player = state.document.create_element("video")?;
    player.set_attribute("id", "karaoke-player")?;
    player.set_attribute("controls", "")?;
    player.set_attribute("playsinline", "")?;
    state.root.append_child(&player)?;
    for (index, video) in state.videos.iter().enumerate() {
        let button = element(
            &state.document,
            "button",
            &format!("{} — {}", video.title, video.artist),
        )?;
        button.set_attribute("data-video", &index.to_string())?;
        state.root.append_child(&button)?;
    }
    Ok(())
}

async fn browse(app: Rc<RefCell<App>>, kind: String) -> Result<(), JsValue> {
    let response = api_request(format!("/api/{kind}"), "GET".into(), None).await?;
    if !response.ok() {
        return Err(JsValue::from_str("Não foi possível carregar os itens"));
    }
    let data = JsFuture::from(response.json()?).await?;
    let values: Vec<serde_json::Value> =
        serde_wasm_bindgen::from_value(data).map_err(|e| JsValue::from_str(&e.to_string()))?;
    let state = app.borrow();
    render_library(&state)?;
    for value in values {
        let Some(id) = value.get("id").and_then(|v| v.as_str()) else {
            continue;
        };
        let name = value.get("name").and_then(|v| v.as_str()).unwrap_or("");
        let button = element(&state.document, "button", name)?;
        button.set_attribute("data-browse-kind", &kind)?;
        button.set_attribute("data-browse-id", id)?;
        state.root.append_child(&button)?;
    }
    Ok(())
}

async fn browse_detail(app: Rc<RefCell<App>>, kind: String, id: String) -> Result<(), JsValue> {
    let response = api_request(
        format!("/api/{kind}/{}", js_sys::encode_uri_component(&id)),
        "GET".into(),
        None,
    )
    .await?;
    if !response.ok() {
        return Err(JsValue::from_str("Não foi possível abrir o item"));
    }
    let data = JsFuture::from(response.json()?).await?;
    let key = if kind == "artists" {
        "topSongs"
    } else {
        "songs"
    };
    let songs = js_sys::Reflect::get(&data, &key.into())?;
    let songs: Vec<Song> = if songs.is_undefined() || songs.is_null() {
        Vec::new()
    } else {
        serde_wasm_bindgen::from_value(songs).map_err(|e| JsValue::from_str(&e.to_string()))?
    };
    let mut state = app.borrow_mut();
    state.songs = songs;
    render_library(&state)
}

async fn playlists(app: Rc<RefCell<App>>) -> Result<(), JsValue> {
    let response = api_request("/api/playlists".into(), "GET".into(), None).await?;
    if !response.ok() {
        return Err(JsValue::from_str("Não foi possível carregar as playlists"));
    }
    let data = JsFuture::from(response.json()?).await?;
    let playlists: Vec<Playlist> =
        serde_wasm_bindgen::from_value(data).map_err(|e| JsValue::from_str(&e.to_string()))?;
    let state = app.borrow();
    render_library(&state)?;
    state
        .root
        .append_child(&element(&state.document, "h2", "Playlists")?)?;
    let create = element(&state.document, "button", "Criar playlist")?;
    create.set_attribute("data-action", "create-playlist")?;
    state.root.append_child(&create)?;
    for list in playlists {
        let row = state.document.create_element("div")?;
        row.append_child(&element(&state.document, "span", &list.name)?)?;
        for (kind, label) in [
            ("data-playlist", "Abrir"),
            ("data-rename", "Renomear"),
            ("data-add", "Adicionar música atual"),
            ("data-delete", "Excluir"),
        ] {
            let button = element(&state.document, "button", label)?;
            button.set_attribute(kind, &list.id)?;
            row.append_child(&button)?;
        }
        state.root.append_child(&row)?;
    }
    Ok(())
}

async fn playlist_detail(app: Rc<RefCell<App>>, id: String) -> Result<(), JsValue> {
    let response = api_request(
        format!("/api/playlists/{}", js_sys::encode_uri_component(&id)),
        "GET".into(),
        None,
    )
    .await?;
    if !response.ok() {
        return Err(JsValue::from_str("Não foi possível abrir a playlist"));
    }
    let data = JsFuture::from(response.json()?).await?;
    let list: Playlist =
        serde_wasm_bindgen::from_value(data).map_err(|e| JsValue::from_str(&e.to_string()))?;
    let mut state = app.borrow_mut();
    state.songs = list.songs.iter().map(|entry| entry.song.clone()).collect();
    render_library(&state)?;
    state
        .root
        .append_child(&element(&state.document, "h2", &list.name)?)?;
    for entry in list.songs {
        let row = state.document.create_element("div")?;
        row.append_child(&element(&state.document, "span", &entry.song.title)?)?;
        let button = element(&state.document, "button", "Remover da playlist")?;
        button.set_attribute("data-remove-playlist", &id)?;
        button.set_attribute("data-entry", &entry.entry_id)?;
        row.append_child(&button)?;
        state.root.append_child(&row)?;
    }
    Ok(())
}

async fn change_playlist(app: Rc<RefCell<App>>, path: String, method: &str, body: Option<String>) {
    match api_request(path, method.into(), body).await {
        Ok(response) if response.ok() => {
            if let Err(error) = playlists(app.clone()).await {
                show_error(
                    &app.borrow(),
                    &error
                        .as_string()
                        .unwrap_or_else(|| "Erro ao carregar playlists".into()),
                );
            }
        }
        _ => show_error(&app.borrow(), "Não foi possível alterar a playlist."),
    }
}

#[derive(serde::Deserialize)]
#[serde(rename_all = "camelCase")]
struct SavedQueue {
    #[serde(default)]
    songs: Vec<Song>,
    #[serde(default)]
    current_index: usize,
    #[serde(default)]
    shuffle: bool,
    #[serde(default)]
    repeat: bool,
}

async fn restore_queue(app: Rc<RefCell<App>>) {
    let Ok(response) = api_request("/api/queue".into(), "GET".into(), None).await else {
        return;
    };
    if !response.ok() {
        return;
    }
    let Ok(promise) = response.json() else { return };
    let Ok(data) = JsFuture::from(promise).await else {
        return;
    };
    let Ok(saved) = serde_wasm_bindgen::from_value::<SavedQueue>(data) else {
        return;
    };
    if saved.current_index >= saved.songs.len() {
        return;
    }
    let mut state = app.borrow_mut();
    let song = &saved.songs[saved.current_index];
    if let Ok(url) = stream_url("", &song.id, &song.format) {
        state.player.load(&url);
    }
    state.current = Some(saved.current_index);
    state.queue = saved.songs;
    state.shuffle = saved.shuffle;
    state.repeat = saved.repeat;
}

async fn login(app: Rc<RefCell<App>>, credentials: serde_json::Value) -> Result<(), JsValue> {
    let body = credentials.to_string();
    let response = api_request("/auth/login".into(), "POST".into(), Some(body)).await?;
    if !response.ok() {
        return Err(JsValue::from_str(
            "Não foi possível entrar. Confira seu telefone.",
        ));
    }
    let value = JsFuture::from(response.json()?).await?;
    let token = js_sys::Reflect::get(&value, &JsValue::from_str("token"))?
        .as_string()
        .ok_or_else(|| JsValue::from_str("Resposta de login inválida"))?;
    set_token(Some(token))?;
    restore_queue(app.clone()).await;
    library(app).await
}

#[wasm_bindgen]
pub fn start_experimental_app() -> Result<(), JsValue> {
    let document = web_sys::window()
        .and_then(|w| w.document())
        .ok_or_else(|| JsValue::from_str("Documento indisponível"))?;
    let root = document
        .get_element_by_id("app")
        .ok_or_else(|| JsValue::from_str("Elemento app ausente"))?;
    let app = Rc::new(RefCell::new(App {
        document,
        root: root.clone(),
        player: BrowserPlayer::new()?,
        songs: Vec::new(),
        current: None,
        queue: Vec::new(),
        shuffle: false,
        repeat: false,
        videos: Vec::new(),
    }));
    render_login(&app.borrow())?;
    install_media_session(app.clone());
    let handler_app = app.clone();
    let handler = Closure::<dyn FnMut(Event)>::new(move |event: Event| {
        let Some(target) = event.target().and_then(|t| t.dyn_into::<Element>().ok()) else {
            return;
        };
        if let Some(index) = target
            .get_attribute("data-song")
            .and_then(|s| s.parse::<usize>().ok())
        {
            let mut state = handler_app.borrow_mut();
            state.queue = state.songs.clone();
            play_track(&mut state, index);
            return;
        }
        if let Some(index) = target
            .get_attribute("data-like")
            .and_then(|s| s.parse::<usize>().ok())
        {
            let song = handler_app.borrow().songs.get(index).cloned();
            if let Some(song) = song {
                let app = handler_app.clone();
                spawn_local(async move {
                    let method = if song.liked { "DELETE" } else { "PUT" };
                    let path = format!("/api/me/liked/{}", song.id);
                    match api_request(path, method.into(), None).await {
                        Ok(response) if response.ok() => {
                            let mut state = app.borrow_mut();
                            if let Some(found) =
                                state.songs.iter_mut().find(|item| item.id == song.id)
                            {
                                found.liked = !song.liked;
                            }
                            let _ = render_library(&state);
                        }
                        _ => show_error(&app.borrow(), "Não foi possível atualizar a curtida."),
                    }
                });
            }
            return;
        }
        if let Some(index) = target
            .get_attribute("data-video")
            .and_then(|s| s.parse::<usize>().ok())
        {
            let state = handler_app.borrow();
            if let Some(video) = state.videos.get(index) {
                let _ = state.player.pause();
                if let Some(node) = state.document.get_element_by_id("karaoke-player") {
                    let params = web_sys::UrlSearchParams::new().ok();
                    if let Some(params) = params {
                        if let Ok(Some(token)) = get_token() {
                            params.set("jwt", &token);
                        }
                        let url = format!(
                            "/api/karaoke/stream/{}?{}",
                            js_sys::encode_uri_component(&video.id),
                            String::from(params.to_string())
                        );
                        let _ = node.set_attribute("src", &url);
                    }
                }
                let id = video.id.clone();
                spawn_local(async move {
                    let _ = api_request(format!("/api/me/karaoke/{id}"), "POST".into(), None).await;
                });
            }
            return;
        }
        if let Some(id) = target.get_attribute("data-playlist") {
            let app = handler_app.clone();
            spawn_local(async move {
                if let Err(e) = playlist_detail(app.clone(), id).await {
                    show_error(&app.borrow(), &e.as_string().unwrap_or_default());
                }
            });
            return;
        }
        if let Some(id) = target.get_attribute("data-rename") {
            let name = web_sys::window().and_then(|w| {
                w.prompt_with_message("Novo nome da playlist")
                    .ok()
                    .flatten()
            });
            if let Some(name) = name.filter(|s| !s.trim().is_empty()) {
                let app = handler_app.clone();
                let body = serde_json::json!({"name": name}).to_string();
                spawn_local(async move {
                    change_playlist(
                        app,
                        format!("/api/playlists/{}", js_sys::encode_uri_component(&id)),
                        "PUT",
                        Some(body),
                    )
                    .await;
                });
            }
            return;
        }
        if let Some(id) = target.get_attribute("data-delete") {
            if web_sys::window()
                .and_then(|w| w.confirm_with_message("Excluir esta playlist?").ok())
                .unwrap_or(false)
            {
                let app = handler_app.clone();
                spawn_local(async move {
                    change_playlist(
                        app,
                        format!("/api/playlists/{}", js_sys::encode_uri_component(&id)),
                        "DELETE",
                        None,
                    )
                    .await;
                });
            }
            return;
        }
        if let Some(id) = target.get_attribute("data-add") {
            let song = {
                let state = handler_app.borrow();
                state.current.and_then(|i| state.queue.get(i)).cloned()
            };
            if let Some(song) = song {
                let app = handler_app.clone();
                let body = serde_json::json!({"songIds": [song.id]}).to_string();
                spawn_local(async move {
                    change_playlist(
                        app,
                        format!(
                            "/api/playlists/{}/tracks",
                            js_sys::encode_uri_component(&id)
                        ),
                        "POST",
                        Some(body),
                    )
                    .await;
                });
            } else {
                show_error(
                    &handler_app.borrow(),
                    "Escolha uma música antes de adicioná-la.",
                );
            }
            return;
        }
        if let Some(id) = target.get_attribute("data-remove-playlist") {
            if let Some(entry) = target.get_attribute("data-entry") {
                let app = handler_app.clone();
                spawn_local(async move {
                    change_playlist(
                        app,
                        format!(
                            "/api/playlists/{}/tracks/{}",
                            js_sys::encode_uri_component(&id),
                            js_sys::encode_uri_component(&entry)
                        ),
                        "DELETE",
                        None,
                    )
                    .await;
                });
            }
            return;
        }
        if let (Some(kind), Some(id)) = (
            target.get_attribute("data-browse-kind"),
            target.get_attribute("data-browse-id"),
        ) {
            let app = handler_app.clone();
            spawn_local(async move {
                if let Err(e) = browse_detail(app.clone(), kind, id).await {
                    show_error(&app.borrow(), &e.as_string().unwrap_or_default());
                }
            });
            return;
        }
        let Some(action) = target.get_attribute("data-action") else {
            return;
        };
        match action.as_str() {
            "login" => {
                let phone = handler_app
                    .borrow()
                    .document
                    .get_element_by_id("phone")
                    .and_then(|e| e.dyn_into::<HtmlInputElement>().ok())
                    .map(|e| e.value())
                    .unwrap_or_default();
                let app = handler_app.clone();
                spawn_local(async move {
                    if let Err(error) =
                        login(app.clone(), serde_json::json!({"phone": phone})).await
                    {
                        show_error(
                            &app.borrow(),
                            &error
                                .as_string()
                                .unwrap_or_else(|| "Erro de conexão".into()),
                        );
                    }
                });
            }
            "login-admin" => {
                let state = handler_app.borrow();
                let value = |id: &str| {
                    state
                        .document
                        .get_element_by_id(id)
                        .and_then(|e| e.dyn_into::<HtmlInputElement>().ok())
                        .map(|e| e.value())
                        .unwrap_or_default()
                };
                let username = value("username");
                let password = value("password");
                drop(state);
                let app = handler_app.clone();
                spawn_local(async move {
                    if let Err(error) = login(
                        app.clone(),
                        serde_json::json!({"username": username, "password": password}),
                    )
                    .await
                    {
                        show_error(
                            &app.borrow(),
                            &error
                                .as_string()
                                .unwrap_or_else(|| "Erro de conexão".into()),
                        );
                    }
                });
            }
            "refresh" => {
                let app = handler_app.clone();
                spawn_local(async move {
                    if let Err(error) = library(app.clone()).await {
                        show_error(
                            &app.borrow(),
                            &error
                                .as_string()
                                .unwrap_or_else(|| "Erro de conexão".into()),
                        );
                    }
                });
            }
            "liked" | "history" | "search" => {
                let app = handler_app.clone();
                let search = action == "search";
                let path = if search {
                    let value = app
                        .borrow()
                        .document
                        .get_element_by_id("search")
                        .and_then(|e| e.dyn_into::<HtmlInputElement>().ok())
                        .map(|e| e.value())
                        .unwrap_or_default();
                    format!(
                        "/api/search?q={}&type=all",
                        js_sys::encode_uri_component(&value)
                    )
                } else if action == "liked" {
                    "/api/me/liked".into()
                } else {
                    "/api/me/history".into()
                };
                spawn_local(async move {
                    if let Err(error) = songs_from(app.clone(), path, search).await {
                        show_error(
                            &app.borrow(),
                            &error
                                .as_string()
                                .unwrap_or_else(|| "Erro de conexão".into()),
                        );
                    }
                });
            }
            "categories" | "albums" | "artists" => {
                let app = handler_app.clone();
                spawn_local(async move {
                    if let Err(e) = browse(app.clone(), action).await {
                        show_error(&app.borrow(), &e.as_string().unwrap_or_default());
                    }
                });
            }
            "playlists" => {
                let app = handler_app.clone();
                spawn_local(async move {
                    if let Err(e) = playlists(app.clone()).await {
                        show_error(&app.borrow(), &e.as_string().unwrap_or_default());
                    }
                });
            }
            "create-playlist" => {
                let name = web_sys::window()
                    .and_then(|w| w.prompt_with_message("Nome da playlist").ok().flatten());
                if let Some(name) = name.filter(|s| !s.trim().is_empty()) {
                    let app = handler_app.clone();
                    let body = serde_json::json!({"name": name, "songIds": []}).to_string();
                    spawn_local(async move {
                        change_playlist(app, "/api/playlists".into(), "POST", Some(body)).await;
                    });
                }
            }
            "karaoke" => {
                let app = handler_app.clone();
                spawn_local(async move {
                    if let Err(error) = karaoke(app.clone()).await {
                        show_error(
                            &app.borrow(),
                            &error
                                .as_string()
                                .unwrap_or_else(|| "Erro de conexão".into()),
                        );
                    }
                });
            }
            "logout" => {
                let mut state = handler_app.borrow_mut();
                let _ = state.player.pause();
                state.player.load("");
                state.queue.clear();
                state.songs.clear();
                state.videos.clear();
                state.current = None;
                let _ = set_token(None);
                let _ = render_login(&state);
            }
            "previous" | "next" => {
                let mut state = handler_app.borrow_mut();
                if let Some(index) = state.current {
                    let new_index = if action == "next" {
                        next_index(&state, index)
                    } else {
                        Some(index.saturating_sub(1))
                    };
                    if let Some(new_index) = new_index {
                        play_track(&mut state, new_index);
                    }
                }
            }
            "shuffle" => {
                let mut state = handler_app.borrow_mut();
                state.shuffle = !state.shuffle;
                show_error(
                    &state,
                    if state.shuffle {
                        "Modo aleatório ativado."
                    } else {
                        "Modo aleatório desativado."
                    },
                );
            }
            "repeat" => {
                let mut state = handler_app.borrow_mut();
                state.repeat = !state.repeat;
                show_error(
                    &state,
                    if state.repeat {
                        "Repetição ativada."
                    } else {
                        "Repetição desativada."
                    },
                );
            }
            "pause" => {
                let _ = handler_app.borrow().player.pause();
            }
            "resume" => {
                let _ = handler_app.borrow().player.element().play();
            }
            _ => {}
        }
    });
    root.add_event_listener_with_callback("click", handler.as_ref().unchecked_ref())?;
    handler.forget();
    for event_name in ["play", "pause", "timeupdate", "durationchange"] {
        let event_app = app.clone();
        let callback =
            Closure::<dyn FnMut(Event)>::new(move |_| sync_media_state(&event_app.borrow()));
        app.borrow()
            .player
            .element()
            .add_event_listener_with_callback(event_name, callback.as_ref().unchecked_ref())?;
        callback.forget();
    }
    let ended_app = app.clone();
    let ended = Closure::<dyn FnMut(Event)>::new(move |_| {
        let mut state = ended_app.borrow_mut();
        if let Some(index) = state.current {
            if let Some(next) = next_index(&state, index) {
                play_track(&mut state, next);
            }
        }
    });
    app.borrow()
        .player
        .element()
        .add_event_listener_with_callback("ended", ended.as_ref().unchecked_ref())?;
    ended.forget();
    if get_token()?.is_some() {
        spawn_local(async move {
            restore_queue(app.clone()).await;
            if library(app.clone()).await.is_err() {
                let _ = render_login(&app.borrow());
            }
        });
    }
    Ok(())
}
