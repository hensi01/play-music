mod api;
mod login;
mod media;
mod routes;
mod store;
use anyhow::Context;
use axum::{
    body::Body,
    extract::{Request, State},
    http::{header, Method, StatusCode},
    response::Response,
    routing::any,
    Router,
};
use rust_embed::RustEmbed;
use sqlx::{postgres::PgPoolOptions, Executor, PgPool};
use std::{env, time::Duration};
use tower_http::cors::{Any, CorsLayer};

#[derive(RustEmbed)]
#[folder = "../web/assets/"]
struct Assets;

#[derive(Clone)]
struct AppState {
    frontend_base_url: String,
    store_config: String,
    pool: PgPool,
    jwt_secret: String,
}

const MIGRATIONS: &[(&str, &str)] = &[
    (
        "0001_init.sql",
        include_str!("../../internal/db/migrations/0001_init.sql"),
    ),
    (
        "0002_users_categories.sql",
        include_str!("../../internal/db/migrations/0002_users_categories.sql"),
    ),
    (
        "0003_categories_songs.sql",
        include_str!("../../internal/db/migrations/0003_categories_songs.sql"),
    ),
    (
        "0004_unaccent.sql",
        include_str!("../../internal/db/migrations/0004_unaccent.sql"),
    ),
    (
        "0005_drop_lyrics.sql",
        include_str!("../../internal/db/migrations/0005_drop_lyrics.sql"),
    ),
    (
        "0006_category_checkout.sql",
        include_str!("../../internal/db/migrations/0006_category_checkout.sql"),
    ),
    (
        "0007_user_email.sql",
        include_str!("../../internal/db/migrations/0007_user_email.sql"),
    ),
    (
        "0008_delete_music_category.sql",
        include_str!("../../internal/db/migrations/0008_delete_music_category.sql"),
    ),
    (
        "0009_karaokes.sql",
        include_str!("../../internal/db/migrations/0009_karaokes.sql"),
    ),
];

// Same version table as the Go service. Transaction-level lock and migration
// transactions ensure that failed migrations are rolled back safely.
async fn migrate(pool: &PgPool) -> anyhow::Result<()> {
    let mut tx = pool.begin().await?;
    sqlx::query("SELECT pg_advisory_xact_lock($1)")
        .bind(0x504d474di64)
        .execute(&mut *tx)
        .await?;
    sqlx::query("CREATE TABLE IF NOT EXISTS schema_migrations (version TEXT PRIMARY KEY, applied_at TIMESTAMPTZ NOT NULL DEFAULT now())").execute(&mut *tx).await?;
    for (name, sql) in MIGRATIONS {
        let exists: bool =
            sqlx::query_scalar("SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version = $1)")
                .bind(name)
                .fetch_one(&mut *tx)
                .await?;
        if !exists {
            sqlx::raw_sql(sql)
                .execute(&mut *tx)
                .await
                .with_context(|| format!("applying migration {name}"))?;
            sqlx::query("INSERT INTO schema_migrations(version) VALUES ($1)")
                .bind(name)
                .execute(&mut *tx)
                .await?;
            tracing::info!(version = name, "migration applied");
        }
    }
    tx.commit().await?;
    Ok(())
}

fn json_string_html_safe(value: &str) -> String {
    serde_json::to_string(value)
        .unwrap()
        .replace('<', "\\u003c")
        .replace('>', "\\u003e")
        .replace('&', "\\u0026")
}

fn response(status: StatusCode, mime: &str, body: Vec<u8>) -> Response {
    Response::builder()
        .status(status)
        .header(header::CONTENT_TYPE, mime)
        .header(header::CACHE_CONTROL, "no-cache")
        .body(Body::from(body))
        .unwrap()
}

async fn static_handler(State(state): State<AppState>, req: Request) -> Response {
    if req.method() != Method::GET && req.method() != Method::HEAD {
        return response(
            StatusCode::METHOD_NOT_ALLOWED,
            "text/plain; charset=utf-8",
            b"method not allowed\n".to_vec(),
        );
    }
    let raw = req.uri().path().trim_start_matches('/');
    if raw == "api" || raw.starts_with("api/") || raw == "auth" || raw.starts_with("auth/") {
        return response(
            StatusCode::NOT_FOUND,
            "application/json",
            "{\"error\":\"Não encontrado\"}\n".as_bytes().to_vec(),
        );
    }
    // Embedded assets are addressed by exact key; traversal does not resolve.
    let path = if raw.is_empty() { "index.html" } else { raw };
    let asset = Assets::get(path);
    if asset.is_none() && (path == "assets" || path.starts_with("assets/")) {
        return response(
            StatusCode::NOT_FOUND,
            "text/plain; charset=utf-8",
            b"404 page not found\n".to_vec(),
        );
    }
    let actual = if asset.is_some() { path } else { "index.html" };
    let bytes = asset
        .or_else(|| Assets::get("index.html"))
        .expect("embedded index.html")
        .data;
    let mut body = bytes.into_owned();
    if actual == "index.html" || actual == "loja.html" {
        let mut html = String::from_utf8(body).expect("HTML UTF-8");
        if actual == "index.html" {
            html = html.replace("__ASSET_VERSION__", "1.16.0").replace(
                "__FRONTEND_BASEURL__",
                &json_string_html_safe(&state.frontend_base_url),
            );
        } else {
            let valid = serde_json::from_str::<serde_json::Value>(&state.store_config).is_ok();
            html = html.replace(
                "__STORE_CONFIG__",
                if valid { &state.store_config } else { "{}" },
            );
        }
        body = html.into_bytes();
    }
    response(
        StatusCode::OK,
        mime_guess::from_path(actual)
            .first_or_octet_stream()
            .as_ref(),
        body,
    )
}

#[tokio::main]
async fn main() -> anyhow::Result<()> {
    tracing_subscriber::fmt()
        .with_env_filter(tracing_subscriber::EnvFilter::from_default_env())
        .init();
    anyhow::ensure!(matches!(env::var("PLAY_MUSIC_RUST_EXPERIMENTAL").as_deref(), Ok("1")), "Backend Rust incompleto: não usar em produção; para testes isolados defina PLAY_MUSIC_RUST_EXPERIMENTAL=1");
    let database_url = env::var("DATABASE_URL").context("DATABASE_URL não configurada")?;
    let pool = PgPoolOptions::new()
        .max_connections(10)
        .acquire_timeout(Duration::from_secs(10))
        .connect(&database_url)
        .await
        .context("falha ao conectar no postgres")?;
    migrate(&pool).await?;
    let jwt_secret = login::init(&pool).await?;
    let state = AppState {
        pool,
        jwt_secret,
        frontend_base_url: env::var("ND_FRONTEND_BASEURL").unwrap_or_default(),
        store_config: env::var("ND_STORE_CONFIG_JSON").unwrap_or_default(),
    };
    let cors = CorsLayer::new()
        .allow_origin(Any)
        .allow_methods(Any)
        .allow_headers(Any)
        .expose_headers([header::HeaderName::from_static("x-nd-authorization")]);
    let protected = Router::new()
        .route("/api/me", axum::routing::get(api::me))
        .route("/api/albums", axum::routing::get(routes::albums))
        .route("/api/albums/:id", axum::routing::get(routes::album))
        .route("/api/artists", axum::routing::get(routes::artists))
        .route("/api/artists/:id", axum::routing::get(routes::artist))
        .route(
            "/api/playlists/:id/tracks",
            axum::routing::post(routes::add_tracks).put(routes::reorder_tracks),
        )
        .route(
            "/api/playlists/:id/tracks/:entry",
            axum::routing::delete(routes::remove_track),
        )
        .route("/api/songs", axum::routing::get(routes::songs))
        .route("/api/songs/:id", axum::routing::get(routes::song))
        .route("/api/karaokes", axum::routing::get(routes::karaokes))
        .route("/api/karaokes/:id", axum::routing::get(routes::karaoke))
        .route("/api/categories", axum::routing::get(routes::categories))
        .route("/api/categories/:id", axum::routing::get(routes::category))
        .route(
            "/api/playlists",
            axum::routing::get(routes::playlists).post(routes::create_playlist),
        )
        .route(
            "/api/playlists/:id",
            axum::routing::get(routes::playlist)
                .put(routes::update_playlist)
                .delete(routes::delete_playlist),
        )
        .route("/api/me/liked", axum::routing::get(routes::liked))
        .route(
            "/api/me/liked/:id",
            axum::routing::put(routes::like).delete(routes::unlike),
        )
        .route("/api/me/history", axum::routing::get(routes::history))
        .route(
            "/api/me/history/:id",
            axum::routing::post(routes::register_play),
        )
        .route(
            "/api/queue",
            axum::routing::get(routes::queue).put(routes::save_queue),
        )
        .route("/api/settings", axum::routing::get(api::settings))
        .route("/api/stream/:id", axum::routing::get(media::song))
        .route(
            "/api/karaoke/stream/:id",
            axum::routing::get(media::karaoke),
        )
        .route_layer(axum::middleware::from_fn_with_state(
            state.clone(),
            api::require_auth,
        ));
    let app = Router::new()
        .merge(protected)
        .route("/auth/login", axum::routing::post(login::login))
        .fallback(any(static_handler))
        .with_state(state)
        .layer(cors);
    let host = env::var("ND_ADDRESS")
        .ok()
        .filter(|v| !v.is_empty())
        .unwrap_or_else(|| "0.0.0.0".into());
    let port = env::var("ND_PORT")
        .ok()
        .and_then(|v| v.parse::<u16>().ok())
        .unwrap_or(4533);
    let listener = tokio::net::TcpListener::bind((host.as_str(), port)).await?;
    tracing::info!(address = %listener.local_addr()?, "Rust backend started (incomplete; not production ready)");
    axum::serve(listener, app).await?;
    Ok(())
}

#[cfg(test)]
mod tests {
    use super::*;
    #[test]
    fn config_string_is_safe_inside_script() {
        assert_eq!(
            json_string_html_safe("</script>"),
            "\"\\u003c/script\\u003e\""
        );
    }
    #[test]
    fn migrations_match_go_versions() {
        assert_eq!(MIGRATIONS.len(), 9);
        assert_eq!(MIGRATIONS[8].0, "0009_karaokes.sql");
    }
}
