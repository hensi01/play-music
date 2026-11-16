use crate::AppState;
use axum::{extract::State, http::StatusCode, response::IntoResponse, Json};
use bcrypt::verify;
use jsonwebtoken::{encode, EncodingKey, Header};
use serde::{Deserialize, Serialize};
use serde_json::json;
use sqlx::{PgPool, Row};
use std::time::{SystemTime, UNIX_EPOCH};

#[derive(Deserialize)]
pub struct LoginRequest {
    #[serde(default)]
    username: String,
    #[serde(default)]
    phone: String,
    #[serde(default)]
    password: String,
}

#[derive(Clone, Serialize, Deserialize)]
pub struct Claims {
    pub uid: String,
    #[serde(skip_serializing_if = "String::is_empty", default)]
    pub username: String,
    #[serde(skip_serializing_if = "String::is_empty", default)]
    pub phone: String,
    pub name: String,
    #[serde(rename = "isAdmin")]
    pub is_admin: bool,
    pub sub: String,
    pub iat: usize,
    pub exp: usize,
}

pub async fn init(pool: &PgPool) -> anyhow::Result<String> {
    use rand::RngCore;
    let mut bytes = [0u8; 32];
    rand::rngs::OsRng.fill_bytes(&mut bytes);
    let secret = hex::encode(bytes);
    // Do not overwrite an existing key: that would invalidate existing sessions.
    sqlx::query(
        "INSERT INTO settings (key, value) VALUES ('jwt_secret', $1) ON CONFLICT (key) DO NOTHING",
    )
    .bind(&secret)
    .execute(pool)
    .await?;
    let secret: String = sqlx::query_scalar("SELECT value FROM settings WHERE key='jwt_secret'")
        .fetch_one(pool)
        .await?;
    anyhow::ensure!(!secret.is_empty(), "JWT secret vazio");
    let exists: bool = sqlx::query_scalar("SELECT EXISTS(SELECT 1 FROM users WHERE is_admin)")
        .fetch_one(pool)
        .await?;
    if !exists {
        let username = std::env::var("ND_ADMINUSERNAME").unwrap_or_default();
        let password = std::env::var("ND_ADMINPASSWORD").unwrap_or_default();
        anyhow::ensure!(!username.is_empty() && !password.is_empty(), "nenhum administrador existe e ND_ADMINUSERNAME/ND_ADMINPASSWORD não estão configuradas");
        let hash = bcrypt::hash(password, bcrypt::DEFAULT_COST)?;
        let mut id_bytes = [0u8; 16];
        rand::rngs::OsRng.fill_bytes(&mut id_bytes);
        let id = hex::encode(id_bytes);
        let mut tx = pool.begin().await?;
        sqlx::query("SELECT pg_advisory_xact_lock($1)")
            .bind(0x504d4155i64)
            .execute(&mut *tx)
            .await?;
        let exists: bool = sqlx::query_scalar("SELECT EXISTS(SELECT 1 FROM users WHERE is_admin)")
            .fetch_one(&mut *tx)
            .await?;
        if !exists {
            sqlx::query("INSERT INTO users (id, username, name, password_hash, is_admin) VALUES ($1,$2,$2,$3,true)").bind(&id).bind(&username).bind(&hash).execute(&mut *tx).await?;
            for sql in [
                "UPDATE playlists SET user_id=$1 WHERE user_id IS NULL",
                "UPDATE user_likes SET user_id=$1 WHERE user_id IS NULL",
                "UPDATE history SET user_id=$1 WHERE user_id IS NULL",
                "UPDATE play_queue SET user_id=$1 WHERE user_id IS NULL",
            ] {
                sqlx::query(sql).bind(&id).execute(&mut *tx).await?;
            }
        }
        tx.commit().await?;
    }
    Ok(secret)
}

pub async fn login(
    State(state): State<AppState>,
    Json(request): Json<LoginRequest>,
) -> impl IntoResponse {
    let is_phone = !request.phone.is_empty();
    let credential = if is_phone {
        let digits: String = request
            .phone
            .chars()
            .filter(|c| c.is_ascii_digit())
            .collect();
        if !(digits.len() == 10 || digits.len() == 11) || digits.starts_with('0') {
            return (
                StatusCode::UNAUTHORIZED,
                Json(json!({"error": "Telefone não cadastrado"})),
            );
        }
        digits
    } else {
        if request.username.is_empty() || request.password.is_empty() {
            return (
                StatusCode::BAD_REQUEST,
                Json(json!({"error": "Informe usuário e senha"})),
            );
        }
        request.username
    };
    let query = if is_phone {
        "SELECT id, COALESCE(username, '') username, COALESCE(phone, '') phone, name, is_admin, password_hash FROM users WHERE phone=$1"
    } else {
        "SELECT id, COALESCE(username, '') username, COALESCE(phone, '') phone, name, is_admin, password_hash FROM users WHERE username=$1 OR (email=$1 AND email <> '')"
    };
    let row = match sqlx::query(query)
        .bind(credential)
        .fetch_optional(&state.pool)
        .await
    {
        Ok(Some(row)) => row,
        Ok(None) => return invalid(is_phone),
        Err(_) => {
            return (
                StatusCode::INTERNAL_SERVER_ERROR,
                Json(json!({"error":"Erro interno"})),
            )
        }
    };
    let admin: bool = row.get("is_admin");
    if (is_phone && admin)
        || (!is_phone
            && (!admin
                || !verify(
                    &request.password,
                    row.get::<String, _>("password_hash").as_str(),
                )
                .unwrap_or(false)))
    {
        return invalid(is_phone);
    }
    let id: String = row.get("id");
    let username: String = row.get("username");
    let phone: String = row.get("phone");
    let name: String = row.get("name");
    let now = SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .unwrap_or_default()
        .as_secs() as usize;
    let claims = Claims {
        uid: id.clone(),
        sub: id.clone(),
        username: username.clone(),
        phone: phone.clone(),
        name: name.clone(),
        is_admin: admin,
        iat: now,
        exp: now + 86400,
    };
    match encode(
        &Header::default(),
        &claims,
        &EncodingKey::from_secret(state.jwt_secret.as_bytes()),
    ) {
        Ok(token) => (
            StatusCode::OK,
            Json(
                json!({"token":token,"id":id,"username":username,"name":name,"phone":phone,"isAdmin":admin}),
            ),
        ),
        Err(_) => (
            StatusCode::INTERNAL_SERVER_ERROR,
            Json(json!({"error":"Erro interno"})),
        ),
    }
}
fn invalid(phone: bool) -> (StatusCode, Json<serde_json::Value>) {
    (
        StatusCode::UNAUTHORIZED,
        Json(
            json!({"error":if phone { "telefone não cadastrado" } else { "usuário ou senha inválidos" }}),
        ),
    )
}
