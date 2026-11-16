use crate::{login::Claims, store::Store, AppState};
use axum::{
    extract::{Path, State},
    http::StatusCode,
    response::{IntoResponse, Response},
    Extension, Json,
};
use serde_json::{json, Value};
fn access(c: &Claims) -> Option<&str> {
    if c.is_admin {
        None
    } else {
        Some(&c.uid)
    }
}
fn err(status: StatusCode, text: &str) -> Response {
    (status, Json(json!({"error":text}))).into_response()
}
fn list(result: anyhow::Result<Vec<Value>>) -> Response {
    match result {
        Ok(v) => Json(v).into_response(),
        Err(_) => err(StatusCode::INTERNAL_SERVER_ERROR, "Erro interno"),
    }
}
fn one(result: anyhow::Result<Option<Value>>) -> Response {
    match result {
        Ok(Some(v)) => Json(v).into_response(),
        Ok(None) => err(StatusCode::NOT_FOUND, "Não encontrado"),
        Err(_) => err(StatusCode::INTERNAL_SERVER_ERROR, "Erro interno"),
    }
}
pub async fn songs(State(s): State<AppState>, Extension(c): Extension<Claims>) -> Response {
    list(
        Store::new(s.pool)
            .list_songs(&c.uid, access(&c), None)
            .await,
    )
}
pub async fn song(
    State(s): State<AppState>,
    Extension(c): Extension<Claims>,
    Path(id): Path<String>,
) -> Response {
    let store = Store::new(s.pool);
    match store.can_access_song(access(&c), &id).await {
        Ok(false) if !c.is_admin => return err(StatusCode::FORBIDDEN, "Sem permissão"),
        Err(_) => return err(StatusCode::INTERNAL_SERVER_ERROR, "Erro interno"),
        _ => (),
    }
    one(store.song(&c.uid, access(&c), &id).await)
}
pub async fn karaokes(State(s): State<AppState>, Extension(c): Extension<Claims>) -> Response {
    list(Store::new(s.pool).list_karaokes(access(&c), None).await)
}
pub async fn karaoke(
    State(s): State<AppState>,
    Extension(c): Extension<Claims>,
    Path(id): Path<String>,
) -> Response {
    let store = Store::new(s.pool);
    match store.can_access_karaoke(access(&c), &id).await {
        Ok(false) if !c.is_admin => return err(StatusCode::FORBIDDEN, "Sem permissão"),
        Err(_) => return err(StatusCode::INTERNAL_SERVER_ERROR, "Erro interno"),
        _ => (),
    }
    one(store.karaoke(access(&c), &id).await)
}
pub async fn categories(State(s): State<AppState>, Extension(c): Extension<Claims>) -> Response {
    list(Store::new(s.pool).list_categories(access(&c)).await)
}
pub async fn category(
    State(s): State<AppState>,
    Extension(c): Extension<Claims>,
    Path(id): Path<String>,
) -> Response {
    let store = Store::new(s.pool);
    if !c.is_admin {
        match sqlx::query_scalar::<_, bool>(
            "SELECT EXISTS(SELECT 1 FROM user_categories WHERE user_id=$1 AND category_id=$2)",
        )
        .bind(&c.uid)
        .bind(&id)
        .fetch_one(&store.pool)
        .await
        {
            Ok(false) => return err(StatusCode::FORBIDDEN, "Sem permissão"),
            Err(_) => return err(StatusCode::INTERNAL_SERVER_ERROR, "Erro interno"),
            _ => (),
        }
    }
    one(store.category_detail(access(&c), &id).await)
}
pub async fn playlists(State(s): State<AppState>, Extension(c): Extension<Claims>) -> Response {
    list(Store::new(s.pool).playlists(&c.uid, access(&c)).await)
}
pub async fn playlist(
    State(s): State<AppState>,
    Extension(c): Extension<Claims>,
    Path(id): Path<String>,
) -> Response {
    one(Store::new(s.pool).playlist(&c.uid, access(&c), &id).await)
}
pub async fn liked(State(s): State<AppState>, Extension(c): Extension<Claims>) -> Response {
    list(Store::new(s.pool).liked_songs(&c.uid, access(&c)).await)
}
pub async fn history(State(s): State<AppState>, Extension(c): Extension<Claims>) -> Response {
    list(Store::new(s.pool).history(&c.uid, access(&c), 100).await)
}
pub async fn queue(State(s): State<AppState>, Extension(c): Extension<Claims>) -> Response {
    match Store::new(s.pool).get_queue(&c.uid).await {
        Ok(v) => Json(v).into_response(),
        Err(_) => err(StatusCode::INTERNAL_SERVER_ERROR, "Erro interno"),
    }
}
pub async fn save_queue(
    State(s): State<AppState>,
    Extension(c): Extension<Claims>,
    Json(v): Json<Value>,
) -> Response {
    match Store::new(s.pool).save_queue(&c.uid, v).await {
        Ok(()) => StatusCode::NO_CONTENT.into_response(),
        Err(_) => err(StatusCode::INTERNAL_SERVER_ERROR, "Erro interno"),
    }
}
pub async fn like(
    State(s): State<AppState>,
    Extension(c): Extension<Claims>,
    Path(id): Path<String>,
) -> Response {
    set_like(s, c, id, true).await
}
pub async fn unlike(
    State(s): State<AppState>,
    Extension(c): Extension<Claims>,
    Path(id): Path<String>,
) -> Response {
    set_like(s, c, id, false).await
}
async fn set_like(s: AppState, c: Claims, id: String, liked: bool) -> Response {
    let store = Store::new(s.pool);
    match store.can_access_song(access(&c), &id).await {
        Ok(true) => (),
        Ok(false) => return err(StatusCode::FORBIDDEN, "Sem permissão"),
        Err(_) => return err(StatusCode::INTERNAL_SERVER_ERROR, "Erro interno"),
    };
    match store.set_like(&c.uid, "song", &id, liked).await {
        Ok(()) => StatusCode::NO_CONTENT.into_response(),
        Err(_) => err(StatusCode::INTERNAL_SERVER_ERROR, "Erro interno"),
    }
}
pub async fn register_play(
    State(s): State<AppState>,
    Extension(c): Extension<Claims>,
    Path(id): Path<String>,
) -> Response {
    match Store::new(s.pool)
        .register_play(&c.uid, access(&c), &id)
        .await
    {
        Ok(true) => StatusCode::NO_CONTENT.into_response(),
        Ok(false) => err(StatusCode::FORBIDDEN, "Sem permissão"),
        Err(_) => err(StatusCode::INTERNAL_SERVER_ERROR, "Erro interno"),
    }
}

pub async fn albums(State(s): State<AppState>, Extension(c): Extension<Claims>) -> Response {
    list(Store::new(s.pool).list_albums(&c.uid, access(&c)).await)
}
pub async fn album(
    State(s): State<AppState>,
    Extension(c): Extension<Claims>,
    Path(id): Path<String>,
) -> Response {
    one(Store::new(s.pool).album(&c.uid, access(&c), &id).await)
}
pub async fn artists(State(s): State<AppState>, Extension(c): Extension<Claims>) -> Response {
    list(Store::new(s.pool).list_artists(&c.uid, access(&c)).await)
}
pub async fn artist(
    State(s): State<AppState>,
    Extension(c): Extension<Claims>,
    Path(id): Path<String>,
) -> Response {
    one(Store::new(s.pool).artist(&c.uid, access(&c), &id).await)
}
#[derive(serde::Deserialize)]
pub struct PlaylistRequest {
    #[serde(default)]
    name: String,
    #[serde(default)]
    comment: String,
    #[serde(default, rename = "songIds")]
    song_ids: Vec<String>,
}
pub async fn create_playlist(
    State(s): State<AppState>,
    Extension(c): Extension<Claims>,
    Json(req): Json<PlaylistRequest>,
) -> Response {
    let name = req.name.trim();
    if name.is_empty() {
        return err(StatusCode::BAD_REQUEST, "Nome da playlist é obrigatório");
    }
    let store = Store::new(s.pool);
    match store
        .create_playlist(&c.uid, name, &req.comment, &req.song_ids, access(&c))
        .await
    {
        Ok(id) => match store.playlist(&c.uid, access(&c), &id).await {
            Ok(Some(v)) => (StatusCode::CREATED, Json(v)).into_response(),
            _ => err(StatusCode::INTERNAL_SERVER_ERROR, "Erro interno"),
        },
        Err(_) => err(StatusCode::INTERNAL_SERVER_ERROR, "Erro interno"),
    }
}
pub async fn update_playlist(
    State(s): State<AppState>,
    Extension(c): Extension<Claims>,
    Path(id): Path<String>,
    Json(req): Json<PlaylistRequest>,
) -> Response {
    done(
        Store::new(s.pool)
            .update_playlist(
                &c.uid,
                &id,
                if req.name.is_empty() {
                    None
                } else {
                    Some(&req.name)
                },
                if req.comment.is_empty() {
                    None
                } else {
                    Some(&req.comment)
                },
            )
            .await,
    )
}
pub async fn delete_playlist(
    State(s): State<AppState>,
    Extension(c): Extension<Claims>,
    Path(id): Path<String>,
) -> Response {
    done(Store::new(s.pool).delete_playlist(&c.uid, &id).await)
}
pub async fn add_tracks(
    State(s): State<AppState>,
    Extension(c): Extension<Claims>,
    Path(id): Path<String>,
    Json(req): Json<PlaylistRequest>,
) -> Response {
    done(
        Store::new(s.pool)
            .add_playlist_tracks(&c.uid, &id, &req.song_ids, access(&c))
            .await,
    )
}
pub async fn remove_track(
    State(s): State<AppState>,
    Extension(c): Extension<Claims>,
    Path((id, entry)): Path<(String, String)>,
) -> Response {
    done(
        Store::new(s.pool)
            .remove_playlist_track(&c.uid, &id, &entry)
            .await,
    )
}
#[derive(serde::Deserialize)]
pub struct Reorder {
    from: usize,
    to: usize,
}
pub async fn reorder_tracks(
    State(s): State<AppState>,
    Extension(c): Extension<Claims>,
    Path(id): Path<String>,
    Json(req): Json<Reorder>,
) -> Response {
    done(
        Store::new(s.pool)
            .reorder_playlist(&c.uid, &id, req.from, req.to)
            .await,
    )
}
fn done(result: anyhow::Result<bool>) -> Response {
    match result {
        Ok(true) => StatusCode::NO_CONTENT.into_response(),
        Ok(false) => err(StatusCode::NOT_FOUND, "Não encontrado"),
        Err(_) => err(StatusCode::INTERNAL_SERVER_ERROR, "Erro interno"),
    }
}
