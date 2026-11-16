use crate::{login::Claims, AppState};
const PATH_ENCODE: &percent_encoding::AsciiSet = &percent_encoding::NON_ALPHANUMERIC
    .remove(b'-')
    .remove(b'_')
    .remove(b'.')
    .remove(b'~');
use axum::{
    extract::{Path, Query, State},
    http::{header, StatusCode},
    response::{IntoResponse, Response},
    Extension, Json,
};
use base64::{engine::general_purpose::URL_SAFE_NO_PAD, Engine};
use hmac::{Hmac, Mac};
use serde::Deserialize;
use serde_json::json;
use sha2::Sha256;
use std::{
    env,
    time::{Duration, SystemTime, UNIX_EPOCH},
};

#[derive(Deserialize)]
pub struct StreamQuery {
    format: Option<String>,
}
fn error(status: StatusCode, message: &str) -> Response {
    (status, Json(json!({"error":message}))).into_response()
}
fn sign(key: &str, decoded: &str, expires: &str, advanced: bool) -> String {
    if advanced {
        let mut mac = Hmac::<Sha256>::new_from_slice(key.as_bytes()).expect("HMAC key");
        mac.update(decoded.as_bytes());
        mac.update(expires.as_bytes());
        format!(
            "HS256-{}",
            URL_SAFE_NO_PAD.encode(mac.finalize().into_bytes())
        )
    } else {
        use md5::{Digest, Md5};
        let mut digest = Md5::new();
        digest.update(key.as_bytes());
        digest.update(decoded.as_bytes());
        digest.update(expires.as_bytes());
        URL_SAFE_NO_PAD.encode(digest.finalize())
    }
}

fn cdn_url(path: &str) -> anyhow::Result<String> {
    anyhow::ensure!(
        env::var("ND_CDN_ENABLED")
            .unwrap_or_default()
            .eq_ignore_ascii_case("true"),
        "CDN não configurado"
    );
    let base = env::var("ND_CDN_BASEURL")?;
    let key = env::var("ND_CDN_TOKENAUTHKEY")?;
    anyhow::ensure!(!base.is_empty() && !key.is_empty(), "CDN não configurado");
    let prefix = env::var("ND_CDN_PATH_PREFIX").unwrap_or_default();
    let prefix = prefix.trim_matches('/');
    let decoded = if prefix.is_empty() {
        format!("/{path}")
    } else {
        format!("/{prefix}/{path}")
    };
    let ttl = env::var("ND_CDN_TOKENTTL")
        .ok()
        .and_then(|s| humantime::parse_duration(&s).ok())
        .filter(|t| !t.is_zero())
        .unwrap_or(Duration::from_secs(86400));
    let expires =
        (SystemTime::now().duration_since(UNIX_EPOCH)?.as_secs() + ttl.as_secs()).to_string();
    let advanced = env::var("ND_CDN_ADVANCEDAUTH")
        .unwrap_or_default()
        .eq_ignore_ascii_case("true");
    let token = sign(&key, &decoded, &expires, advanced);
    let encoded = decoded
        .split('/')
        .map(|s| percent_encoding::utf8_percent_encode(s, PATH_ENCODE).to_string())
        .collect::<Vec<_>>()
        .join("/");
    Ok(format!(
        "{}{}?token={}&expires={}",
        base.trim_end_matches('/'),
        encoded,
        token,
        expires
    ))
}

pub async fn song(
    State(state): State<AppState>,
    Extension(claims): Extension<Claims>,
    Path(id): Path<String>,
    Query(query): Query<StreamQuery>,
) -> Response {
    let access = claims.is_admin || match sqlx::query_scalar::<_, bool>("SELECT EXISTS(SELECT 1 FROM category_songs cs JOIN user_categories uc ON uc.category_id=cs.category_id WHERE uc.user_id=$1 AND cs.song_id=$2)").bind(&claims.uid).bind(&id).fetch_one(&state.pool).await { Ok(v) => v, Err(_) => return error(StatusCode::INTERNAL_SERVER_ERROR,"Erro interno") };
    if !access {
        return error(StatusCode::FORBIDDEN, "Sem permissão");
    }
    let item = sqlx::query_as::<_, (String, String)>("SELECT path, format FROM songs WHERE id=$1")
        .bind(&id)
        .fetch_optional(&state.pool)
        .await;
    let (path, format) = match item {
        Ok(Some(v)) => v,
        Ok(None) => return error(StatusCode::NOT_FOUND, "Não encontrado"),
        Err(_) => return error(StatusCode::INTERNAL_SERVER_ERROR, "Erro interno"),
    };
    if let Some(requested) = query.format.filter(|v| !v.is_empty() && *v != format) {
        if requested != "mp3" {
            return error(
                StatusCode::BAD_REQUEST,
                "Formato de transcodificação não suportado",
            );
        }
        return error(
            StatusCode::NOT_IMPLEMENTED,
            "Transcodificação ainda não implementada",
        );
    }
    if !["mp3", "m4a", "aac", "ogg", "opus", "wav", "flac"].contains(&format.as_str()) {
        return error(
            StatusCode::NOT_IMPLEMENTED,
            "Transcodificação ainda não implementada",
        );
    }
    match cdn_url(&path) {
        Ok(url) => (StatusCode::FOUND, [(header::LOCATION, url)]).into_response(),
        Err(_) => error(StatusCode::INTERNAL_SERVER_ERROR, "CDN não configurado"),
    }
}
pub async fn karaoke(
    State(state): State<AppState>,
    Extension(claims): Extension<Claims>,
    Path(id): Path<String>,
) -> Response {
    let access = claims.is_admin || match sqlx::query_scalar::<_, bool>("SELECT EXISTS(SELECT 1 FROM category_karaokes ck JOIN user_categories uc ON uc.category_id=ck.category_id WHERE uc.user_id=$1 AND ck.karaoke_id=$2)").bind(&claims.uid).bind(&id).fetch_one(&state.pool).await { Ok(v) => v, Err(_) => return error(StatusCode::INTERNAL_SERVER_ERROR,"Erro interno") };
    if !access {
        return error(StatusCode::FORBIDDEN, "Sem permissão");
    }
    let path = match sqlx::query_scalar::<_, String>("SELECT path FROM karaokes WHERE id=$1")
        .bind(&id)
        .fetch_optional(&state.pool)
        .await
    {
        Ok(Some(v)) => v,
        Ok(None) => return error(StatusCode::NOT_FOUND, "Não encontrado"),
        Err(_) => return error(StatusCode::INTERNAL_SERVER_ERROR, "Erro interno"),
    };
    match cdn_url(&path) {
        Ok(url) => (StatusCode::FOUND, [(header::LOCATION, url)]).into_response(),
        Err(_) => error(StatusCode::INTERNAL_SERVER_ERROR, "CDN não configurado"),
    }
}

#[cfg(test)]
mod tests {
    use super::sign;
    #[test]
    fn signing_matches_existing_go_vectors() {
        let key = "50df94e8-7424-4a4f-99e9-1c136831052d";
        let path = "/Test Artist/Test Album/test.mp3";
        let expiry = "1786072596";
        assert_eq!(
            sign(key, path, expiry, true),
            "HS256-ZQDIooiFFM7aYHPmKb6KYR6jtbTP6cQTZJEOjP1TqbA"
        );
        assert_eq!(sign(key, path, expiry, false), "WOgdbUhlRrRq_pBXwjZuSQ");
    }
}
