//! Browser-facing primitives for the gradual migration of Play Music to Rust.
//! The existing frontend remains the entry point until feature parity is reached.
use wasm_bindgen::prelude::*;
use wasm_bindgen::JsCast;
use wasm_bindgen_futures::JsFuture;
use web_sys::{HtmlAudioElement, Request, RequestInit, Response};

const TOKEN_KEY: &str = "pm_token";
const AUTH_HEADER: &str = "X-ND-Authorization";

fn window() -> Result<web_sys::Window, JsValue> {
    web_sys::window().ok_or_else(|| JsValue::from_str("Browser window unavailable"))
}

#[wasm_bindgen]
pub fn get_token() -> Result<Option<String>, JsValue> {
    match window()?.local_storage()? {
        Some(storage) => storage.get_item(TOKEN_KEY),
        None => Ok(None),
    }
}

#[wasm_bindgen]
pub fn set_token(token: Option<String>) -> Result<(), JsValue> {
    if let Some(storage) = window()?.local_storage()? {
        match token {
            Some(token) if !token.is_empty() => storage.set_item(TOKEN_KEY, &token)?,
            _ => storage.remove_item(TOKEN_KEY)?,
        }
    }
    Ok(())
}

/// Perform an authenticated API request. Returns the original response, so callers
/// can handle HTTP errors and empty responses without losing information.
#[wasm_bindgen]
pub async fn api_request(
    url: String,
    method: String,
    body: Option<String>,
) -> Result<Response, JsValue> {
    if !url.starts_with('/')
        || url.starts_with("//")
        || url.contains('\\')
        || url.chars().any(char::is_control)
    {
        return Err(JsValue::from_str(
            "API path must be relative to the current origin",
        ));
    }
    let init = RequestInit::new();
    init.set_method(&method);
    if let Some(body) = &body {
        init.set_body(&JsValue::from_str(body));
    }
    let request = Request::new_with_str_and_init(&url, &init)?;
    if body.is_some() {
        request.headers().set("Content-Type", "application/json")?;
    }
    if let Some(token) = get_token()? {
        request
            .headers()
            .set(AUTH_HEADER, &format!("Bearer {token}"))?;
    }
    let response: Response = JsFuture::from(window()?.fetch_with_request(&request))
        .await?
        .dyn_into()?;
    if let Some(refreshed) = response.headers().get(AUTH_HEADER)? {
        set_token(Some(refreshed))?;
    }
    if response.status() == 401 {
        set_token(None)?;
        let event = web_sys::Event::new("pm:unauthorized")?;
        window()?.dispatch_event(&event)?;
    }
    Ok(response)
}

/// A browser audio element owned by the Rust player. The surrounding UI remains
/// responsible for queue management until that migration is complete.
#[wasm_bindgen]
pub struct BrowserPlayer {
    audio: HtmlAudioElement,
}

#[wasm_bindgen]
impl BrowserPlayer {
    #[wasm_bindgen(constructor)]
    pub fn new() -> Result<BrowserPlayer, JsValue> {
        let audio = HtmlAudioElement::new()?;
        audio.set_preload("auto");
        audio.set_volume(0.8);
        Ok(Self { audio })
    }

    pub fn load(&self, url: &str) {
        self.audio.set_src(url);
    }

    pub async fn play(&self) -> Result<(), JsValue> {
        JsFuture::from(self.audio.play()?).await?;
        Ok(())
    }

    pub fn pause(&self) -> Result<(), JsValue> {
        self.audio.pause()
    }
    pub fn seek(&self, seconds: f64) {
        if seconds.is_finite() {
            self.audio.set_current_time(seconds.max(0.0));
        }
    }
    pub fn set_volume(&self, volume: f64) {
        if volume.is_finite() {
            self.audio.set_volume(volume.clamp(0.0, 1.0));
        }
    }
    pub fn position(&self) -> f64 {
        self.audio.current_time()
    }
    pub fn duration(&self) -> f64 {
        self.audio.duration()
    }
    pub fn is_paused(&self) -> bool {
        self.audio.paused()
    }
    pub fn element(&self) -> HtmlAudioElement {
        self.audio.clone()
    }
}

#[derive(Clone, serde::Serialize, serde::Deserialize)]
pub struct Song {
    pub id: String,
    #[serde(default)]
    pub title: String,
    #[serde(default)]
    pub artist: String,
    #[serde(default)]
    pub album: String,
    #[serde(default)]
    pub format: String,
    #[serde(default)]
    pub duration: f64,
    #[serde(default)]
    pub liked: bool,
}

/// Reproduces the URL contract used by the existing server, including its
/// authenticated query string and transcoding for non-browser-native formats.
#[wasm_bindgen]
pub fn stream_url(base: &str, id: &str, format: &str) -> Result<String, JsValue> {
    let query = web_sys::UrlSearchParams::new()?;
    if !matches!(
        format.to_ascii_lowercase().as_str(),
        "mp3" | "m4a" | "aac" | "ogg" | "opus" | "wav" | "flac"
    ) {
        query.set("format", "mp3");
    }
    if let Some(token) = get_token()? {
        query.set("jwt", &token);
    }
    let query = String::from(query.to_string());
    let path = format!(
        "{}/api/stream/{}",
        base.trim_end_matches('/'),
        js_sys::encode_uri_component(id)
    );
    Ok(if query.is_empty() {
        path
    } else {
        format!("{path}?{query}")
    })
}

/// Queue state is serializable for UI integrations; it never stores tokens.
#[wasm_bindgen]
pub struct PlaybackQueue {
    songs: Vec<Song>,
    index: Option<usize>,
    repeat: bool,
}

#[wasm_bindgen]
impl PlaybackQueue {
    #[wasm_bindgen(constructor)]
    pub fn new() -> Self {
        Self {
            songs: Vec::new(),
            index: None,
            repeat: false,
        }
    }

    pub fn replace(&mut self, songs: JsValue, index: usize) -> Result<(), JsValue> {
        let songs: Vec<Song> = serde_wasm_bindgen::from_value(songs)
            .map_err(|error| JsValue::from_str(&error.to_string()))?;
        if index >= songs.len() {
            return Err(JsValue::from_str("Invalid queue index"));
        }
        self.songs = songs;
        self.index = Some(index);
        Ok(())
    }

    pub fn current(&self) -> Result<JsValue, JsValue> {
        let song = self.index.and_then(|index| self.songs.get(index));
        serde_wasm_bindgen::to_value(&song).map_err(|error| JsValue::from_str(&error.to_string()))
    }

    pub fn set_repeat(&mut self, enabled: bool) {
        self.repeat = enabled;
    }

    pub fn next(&mut self) -> bool {
        let Some(index) = self.index else {
            return false;
        };
        if index + 1 < self.songs.len() {
            self.index = Some(index + 1);
            true
        } else if self.repeat && !self.songs.is_empty() {
            self.index = Some(0);
            true
        } else {
            false
        }
    }

    pub fn previous(&mut self) -> bool {
        let Some(index) = self.index else {
            return false;
        };
        self.index = Some(index.saturating_sub(1));
        true
    }
}

#[cfg(test)]
mod tests {
    use super::*;

    fn song(id: &str) -> Song {
        Song {
            id: id.into(),
            title: String::new(),
            artist: String::new(),
            album: String::new(),
            format: "mp3".into(),
            duration: 0.0,
            liked: false,
        }
    }

    #[test]
    fn queue_stops_at_end_unless_repeat_is_enabled() {
        let mut queue = PlaybackQueue {
            songs: vec![song("a"), song("b")],
            index: Some(0),
            repeat: false,
        };
        assert!(queue.next());
        assert_eq!(queue.index, Some(1));
        assert!(!queue.next());
        assert_eq!(queue.index, Some(1));
        queue.set_repeat(true);
        assert!(queue.next());
        assert_eq!(queue.index, Some(0));
        assert!(queue.previous());
        assert_eq!(queue.index, Some(0));
    }

    #[test]
    fn empty_queue_never_advances() {
        let mut queue = PlaybackQueue::new();
        queue.set_repeat(true);
        assert!(!queue.next());
        assert!(!queue.previous());
    }
}

mod ui;
