use crate::{login::Claims, AppState};
use axum::{
    extract::{Request, State},
    http::{header, StatusCode},
    middleware::Next,
    response::{IntoResponse, Response},
    Json,
};
use jsonwebtoken::{decode, encode, DecodingKey, EncodingKey, Header, Validation};
use serde_json::{json, Value};
use sqlx::Row;
use std::time::{SystemTime, UNIX_EPOCH};

pub async fn require_auth(State(state): State<AppState>, mut req: Request, next: Next) -> Response {
    let token = req
        .headers()
        .get("x-nd-authorization")
        .or_else(|| req.headers().get(header::AUTHORIZATION))
        .and_then(|v| v.to_str().ok())
        .map(|v| v.strip_prefix("Bearer ").unwrap_or(v).to_owned())
        .or_else(|| {
            req.uri().query().and_then(|q| {
                url::form_urlencoded::parse(q.as_bytes())
                    .find(|(k, _)| k == "jwt")
                    .map(|(_, v)| v.into_owned())
            })
        });
    let claims = token
        .and_then(|t| {
            decode::<Claims>(
                &t,
                &DecodingKey::from_secret(state.jwt_secret.as_bytes()),
                &Validation::default(),
            )
            .ok()
        })
        .map(|v| v.claims);
    let mut claims = match claims {
        Some(c) => c,
        None => {
            return (
                StatusCode::UNAUTHORIZED,
                Json(json!({"error":"Não autenticado"})),
            )
                .into_response()
        }
    };
    let now = SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .unwrap_or_default()
        .as_secs() as usize;
    let refreshed = if claims.exp.saturating_sub(now) < 43200 {
        claims.iat = now;
        claims.exp = now + 86400;
        encode(
            &Header::default(),
            &claims,
            &EncodingKey::from_secret(state.jwt_secret.as_bytes()),
        )
        .ok()
    } else {
        None
    };
    req.extensions_mut().insert(claims);
    let mut res = next.run(req).await;
    if let Some(token) = refreshed.and_then(|v| v.parse().ok()) {
        res.headers_mut().insert("x-nd-authorization", token);
    }
    res
}

pub async fn me(
    State(state): State<AppState>,
    axum::Extension(claims): axum::Extension<Claims>,
) -> Response {
    let row = sqlx::query("SELECT id, username, email, phone, name, is_admin, created_at::text AS created_at FROM users WHERE id=$1").bind(&claims.uid).fetch_optional(&state.pool).await;
    match row {
        Ok(Some(row)) => {
            let mut result = json!({"id":row.get::<String,_>("id"), "name":row.get::<String,_>("name"), "isAdmin":row.get::<bool,_>("is_admin"), "createdAt":row.get::<String,_>("created_at")});
            for key in ["username", "email", "phone"] {
                if let Some(v) = row.get::<Option<String>, _>(key).filter(|s| !s.is_empty()) {
                    result[key] = Value::String(v);
                }
            }
            Json(result).into_response()
        }
        Ok(None) => (
            StatusCode::NOT_FOUND,
            Json(json!({"error":"Não encontrado"})),
        )
            .into_response(),
        Err(_) => (
            StatusCode::INTERNAL_SERVER_ERROR,
            Json(json!({"error":"Erro interno"})),
        )
            .into_response(),
    }
}
pub async fn settings() -> Json<Value> {
    Json(
        json!({"appName":"Play Music", "version":"1.16.0", "libraryName":"Play Music", "musicFolder":std::env::var("ND_MUSICFOLDER").unwrap_or_default()}),
    )
}
