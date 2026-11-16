//! PostgreSQL repository used by the Rust HTTP API.
//!
//! All queries preserve the existing PostgreSQL schema and scope client data
//! through category grants. `access_user` is `None` for an administrator and
//! `Some(user_id)` for a client; `viewer_id` always identifies the user whose
//! likes/history should be returned.
use anyhow::{bail, Result};
use serde_json::Value;
use sqlx::{PgPool, Row};

#[derive(Clone)]
pub struct Store {
    pub pool: PgPool,
}

impl Store {
    pub fn new(pool: PgPool) -> Self {
        Self { pool }
    }

    pub async fn list_songs(
        &self,
        viewer_id: &str,
        access_user: Option<&str>,
        limit: Option<i64>,
    ) -> Result<Vec<Value>> {
        let rows = sqlx::query_scalar::<_, Value>(
            r#"
            SELECT jsonb_build_object(
                'id',s.id,'title',s.title,'artist',s.artist,'artistId',s.artist_id,
                'album',s.album,'albumId',s.album_id,'year',s.year,'genre',s.genre,
                'duration',s.duration,'format',s.format,'bitrate',s.bitrate,
                'sampleRate',s.sample_rate,'trackNumber',s.track_number,
                'discNumber',s.disc_number,'playCount',s.play_count,
                'liked',EXISTS(SELECT 1 FROM user_likes ul WHERE ul.user_id=$1
                    AND ul.entity_type='song' AND ul.entity_id=s.id))
            FROM songs s
            WHERE ($2::text IS NULL OR EXISTS (
                SELECT 1 FROM category_songs cs JOIN user_categories uc
                  ON uc.category_id=cs.category_id
                WHERE cs.song_id=s.id AND uc.user_id=$2))
            ORDER BY s.title COLLATE "C" ASC
            LIMIT COALESCE($3::bigint, 2147483647::bigint)
        "#,
        )
        .bind(viewer_id)
        .bind(access_user)
        .bind(limit)
        .fetch_all(&self.pool)
        .await?;
        Ok(rows)
    }

    pub async fn search_songs(
        &self,
        viewer_id: &str,
        access_user: Option<&str>,
        query: &str,
        limit: i64,
    ) -> Result<Vec<Value>> {
        let like = like_pattern(query);
        Ok(sqlx::query_scalar::<_, Value>(r#"
          SELECT jsonb_build_object('id',s.id,'title',s.title,'artist',s.artist,'artistId',s.artist_id,
            'album',s.album,'albumId',s.album_id,'year',s.year,'genre',s.genre,'duration',s.duration,
            'format',s.format,'bitrate',s.bitrate,'sampleRate',s.sample_rate,'trackNumber',s.track_number,
            'discNumber',s.disc_number,'playCount',s.play_count,
            'liked',EXISTS(SELECT 1 FROM user_likes ul WHERE ul.user_id=$1 AND ul.entity_type='song' AND ul.entity_id=s.id))
          FROM songs s WHERE (unaccent(s.title) ILIKE unaccent($2) ESCAPE '\'
              OR unaccent(s.artist) ILIKE unaccent($2) ESCAPE '\' OR unaccent(s.album) ILIKE unaccent($2) ESCAPE '\')
            AND ($3::text IS NULL OR EXISTS(SELECT 1 FROM category_songs cs JOIN user_categories uc ON uc.category_id=cs.category_id WHERE cs.song_id=s.id AND uc.user_id=$3))
          ORDER BY s.title COLLATE "C" LIMIT $4
        "#).bind(viewer_id).bind(like).bind(access_user).bind(limit).fetch_all(&self.pool).await?)
    }

    pub async fn search_categories(
        &self,
        access_user: Option<&str>,
        query: &str,
        limit: i64,
    ) -> Result<Vec<Value>> {
        let like = like_pattern(query);
        Ok(sqlx::query_scalar::<_, Value>(r#"
          SELECT jsonb_build_object('id',c.id,'name',c.name,
            'songCount',(SELECT count(*) FROM category_songs cs WHERE cs.category_id=c.id),
            'karaokeCount',(SELECT count(*) FROM category_karaokes ck WHERE ck.category_id=c.id))
          FROM categories c WHERE unaccent(c.name) ILIKE unaccent($1) ESCAPE '\'
            AND ($2::text IS NULL OR EXISTS(SELECT 1 FROM user_categories uc WHERE uc.category_id=c.id AND uc.user_id=$2))
          ORDER BY c.name LIMIT $3
        "#).bind(like).bind(access_user).bind(limit).fetch_all(&self.pool).await?)
    }

    pub async fn search_playlists(
        &self,
        user_id: &str,
        query: &str,
        limit: i64,
    ) -> Result<Vec<Value>> {
        let like = like_pattern(query);
        Ok(sqlx::query_scalar::<_, Value>(
            r#"
          SELECT jsonb_build_object('id',p.id,'name',p.name,'comment',p.comment,'owner',p.owner,
            'songCount',(SELECT count(*) FROM playlist_tracks pt WHERE pt.playlist_id=p.id))
          FROM playlists p WHERE p.user_id=$1 AND unaccent(p.name) ILIKE unaccent($2) ESCAPE '\'
          ORDER BY p.name LIMIT $3
        "#,
        )
        .bind(user_id)
        .bind(like)
        .bind(limit)
        .fetch_all(&self.pool)
        .await?)
    }

    pub async fn get_user(&self, id: &str) -> Result<Option<Value>> {
        Ok(sqlx::query_scalar::<_, Value>(r#"
          SELECT jsonb_build_object('id',u.id,'username',u.username,'email',u.email,'phone',u.phone,
            'name',u.name,'isAdmin',u.is_admin,'createdAt',u.created_at)
          FROM users u WHERE u.id=$1
        "#).bind(id).fetch_optional(&self.pool).await?)
    }

    pub async fn list_users(&self) -> Result<Vec<Value>> {
        let mut users = sqlx::query_scalar::<_, Value>(r#"
          SELECT jsonb_build_object('id',u.id,'username',u.username,'email',u.email,'phone',u.phone,
            'name',u.name,'isAdmin',u.is_admin,'createdAt',u.created_at)
          FROM users u ORDER BY u.is_admin DESC,u.name COLLATE "C"
        "#).fetch_all(&self.pool).await?;
        for user in &mut users {
            let id = user["id"].as_str().unwrap_or_default();
            let categories = sqlx::query_scalar::<_, Value>(r#"
              SELECT jsonb_build_object('id',c.id,'name',c.name,
                'songCount',(SELECT count(*) FROM category_songs cs WHERE cs.category_id=c.id))
              FROM user_categories uc JOIN categories c ON c.id=uc.category_id
              WHERE uc.user_id=$1 ORDER BY c.name COLLATE "C"
            "#).bind(id).fetch_all(&self.pool).await?;
            user["categories"] = Value::Array(categories);
        }
        Ok(users)
    }

    pub async fn create_user(&self, id: &str, username: Option<&str>, email: &str, phone: Option<&str>, name: &str, password_hash: &str, is_admin: bool, category_ids: &[String]) -> Result<()> {
        let mut tx = self.pool.begin().await?;
        sqlx::query("INSERT INTO users(id,username,email,phone,name,password_hash,is_admin,created_at) VALUES($1,NULLIF($2,''),$3,NULLIF($4,''),$5,$6,$7,now())")
            .bind(id).bind(username).bind(email).bind(phone).bind(name).bind(password_hash).bind(is_admin).execute(&mut *tx).await?;
        replace_user_categories(&mut tx, id, category_ids).await?;
        tx.commit().await?;
        Ok(())
    }

    /// Partial user update. Outer `None` leaves an optional field unchanged;
    /// `Some(None)` clears nullable username/phone; email is stored as empty.
    pub async fn update_user(
        &self, id: &str, name: Option<&str>, username: Option<Option<&str>>,
        email: Option<Option<&str>>, phone: Option<Option<&str>>,
        password_hash: Option<&str>, is_admin: Option<bool>, category_ids: Option<&[String]>,
    ) -> Result<bool> {
        let mut tx = self.pool.begin().await?;
        let exists: bool = sqlx::query_scalar("SELECT EXISTS(SELECT 1 FROM users WHERE id=$1)")
            .bind(id).fetch_one(&mut *tx).await?;
        if !exists { tx.rollback().await?; return Ok(false); }
        if let Some(v) = name.filter(|s| !s.is_empty()) {
            sqlx::query("UPDATE users SET name=$2 WHERE id=$1").bind(id).bind(v).execute(&mut *tx).await?;
        }
        if let Some(v) = username {
            sqlx::query("UPDATE users SET username=NULLIF($2,'') WHERE id=$1").bind(id).bind(v).execute(&mut *tx).await?;
        }
        if let Some(v) = email {
            sqlx::query("UPDATE users SET email=COALESCE($2,'') WHERE id=$1").bind(id).bind(v).execute(&mut *tx).await?;
        }
        if let Some(v) = phone {
            sqlx::query("UPDATE users SET phone=NULLIF($2,'') WHERE id=$1").bind(id).bind(v).execute(&mut *tx).await?;
        }
        if let Some(v) = password_hash {
            if !v.is_empty() { sqlx::query("UPDATE users SET password_hash=$2 WHERE id=$1").bind(id).bind(v).execute(&mut *tx).await?; }
        }
        if let Some(v) = is_admin {
            sqlx::query("UPDATE users SET is_admin=$2 WHERE id=$1").bind(id).bind(v).execute(&mut *tx).await?;
        }
        if let Some(ids) = category_ids { replace_user_categories(&mut tx, id, ids).await?; }
        tx.commit().await?;
        Ok(true)
    }

    pub async fn delete_user(&self, id: &str) -> Result<bool> {
        let result = sqlx::query("DELETE FROM users WHERE id=$1").bind(id).execute(&self.pool).await?;
        Ok(result.rows_affected() > 0)
    }

    pub async fn category_admin_detail(&self, id: &str) -> Result<Option<Value>> {
        let Some(mut category) = sqlx::query_scalar::<_, Value>(r#"
          SELECT jsonb_build_object('id',c.id,'name',c.name,'checkoutUrl',c.checkout_url,
            'songCount',(SELECT count(*) FROM category_songs cs WHERE cs.category_id=c.id),
            'karaokeCount',(SELECT count(*) FROM category_karaokes ck WHERE ck.category_id=c.id))
          FROM categories c WHERE c.id=$1
        "#).bind(id).fetch_optional(&self.pool).await? else { return Ok(None) };
        let song_ids = sqlx::query_scalar("SELECT song_id FROM category_songs WHERE category_id=$1 ORDER BY position,song_id")
            .bind(id).fetch_all(&self.pool).await?;
        let karaoke_ids = sqlx::query_scalar("SELECT karaoke_id FROM category_karaokes WHERE category_id=$1 ORDER BY position,karaoke_id")
            .bind(id).fetch_all(&self.pool).await?;
        let album_ids = sqlx::query_scalar("SELECT album_id FROM category_albums WHERE category_id=$1 ORDER BY album_id")
            .bind(id).fetch_all(&self.pool).await?;
        let artist_ids = sqlx::query_scalar("SELECT artist_id FROM category_artists WHERE category_id=$1 ORDER BY artist_id")
            .bind(id).fetch_all(&self.pool).await?;
        category["songIds"] = serde_json::to_value(song_ids)?;
        category["karaokeIds"] = serde_json::to_value(karaoke_ids)?;
        category["albumIds"] = serde_json::to_value(album_ids)?;
        category["artistIds"] = serde_json::to_value(artist_ids)?;
        Ok(Some(category))
    }

    pub async fn create_category(&self, name: &str, checkout_url: &str) -> Result<Value> {
        let row = sqlx::query_scalar::<_, Value>(r#"
          WITH ins AS (INSERT INTO categories(id,name,checkout_url,created_at)
            VALUES($1,$2,$3,now()) ON CONFLICT(lower(name)) DO NOTHING RETURNING id)
          SELECT jsonb_build_object('id',c.id,'name',c.name,'checkoutUrl',c.checkout_url,
            'songCount',0,'karaokeCount',0)
          FROM categories c WHERE c.id=(SELECT id FROM ins UNION ALL SELECT id FROM categories WHERE lower(name)=lower($2) LIMIT 1)
        "#).bind(new_id()).bind(name).bind(checkout_url).fetch_one(&self.pool).await?;
        Ok(row)
    }

    pub async fn update_category(
        &self, id: &str, name: Option<&str>, checkout_url: Option<&str>,
        song_ids: Option<&[String]>, karaoke_ids: Option<&[String]>,
        album_ids: Option<&[String]>, artist_ids: Option<&[String]>,
    ) -> Result<bool> {
        let mut tx = self.pool.begin().await?;
        let exists: bool = sqlx::query_scalar("SELECT EXISTS(SELECT 1 FROM categories WHERE id=$1)")
            .bind(id).fetch_one(&mut *tx).await?;
        if !exists { tx.rollback().await?; return Ok(false); }
        if let Some(n) = name.filter(|s| !s.is_empty()) {
            sqlx::query("UPDATE categories SET name=$2 WHERE id=$1").bind(id).bind(n).execute(&mut *tx).await?;
        }
        if let Some(url) = checkout_url {
            sqlx::query("UPDATE categories SET checkout_url=$2 WHERE id=$1").bind(id).bind(url).execute(&mut *tx).await?;
        }
        if let Some(ids) = song_ids { replace_category_ids(&mut tx, "category_songs", "song_id", id, ids).await?; }
        if let Some(ids) = karaoke_ids { replace_category_ids(&mut tx, "category_karaokes", "karaoke_id", id, ids).await?; }
        if let Some(ids) = album_ids { replace_category_ids(&mut tx, "category_albums", "album_id", id, ids).await?; }
        if let Some(ids) = artist_ids { replace_category_ids(&mut tx, "category_artists", "artist_id", id, ids).await?; }
        tx.commit().await?;
        Ok(true)
    }

    pub async fn delete_category(&self, id: &str) -> Result<bool> {
        let result = sqlx::query("DELETE FROM categories WHERE id=$1").bind(id).execute(&self.pool).await?;
        Ok(result.rows_affected() > 0)
    }

    pub async fn song(
        &self,
        viewer_id: &str,
        access_user: Option<&str>,
        id: &str,
    ) -> Result<Option<Value>> {
        Ok(sqlx::query_scalar::<_, Value>(r#"
            SELECT jsonb_build_object('id',s.id,'title',s.title,'artist',s.artist,
                'artistId',s.artist_id,'album',s.album,'albumId',s.album_id,
                'year',s.year,'genre',s.genre,'duration',s.duration,'format',s.format,
                'bitrate',s.bitrate,'sampleRate',s.sample_rate,'trackNumber',s.track_number,
                'discNumber',s.disc_number,'playCount',s.play_count,
                'liked',EXISTS(SELECT 1 FROM user_likes ul WHERE ul.user_id=$1
                    AND ul.entity_type='song' AND ul.entity_id=s.id))
            FROM songs s WHERE s.id=$3 AND ($2::text IS NULL OR EXISTS (
                SELECT 1 FROM category_songs cs JOIN user_categories uc ON uc.category_id=cs.category_id
                WHERE cs.song_id=s.id AND uc.user_id=$2))
        "#).bind(viewer_id).bind(access_user).bind(id).fetch_optional(&self.pool).await?)
    }

    pub async fn list_karaokes(
        &self,
        access_user: Option<&str>,
        limit: Option<i64>,
    ) -> Result<Vec<Value>> {
        Ok(sqlx::query_scalar::<_, Value>(r#"
            SELECT jsonb_build_object('id',k.id,'title',k.title,'artist',k.artist,
                'duration',k.duration,'format',k.format,'playCount',k.play_count)
            FROM karaokes k WHERE ($1::text IS NULL OR EXISTS (
                SELECT 1 FROM category_karaokes ck JOIN user_categories uc ON uc.category_id=ck.category_id
                WHERE ck.karaoke_id=k.id AND uc.user_id=$1))
            ORDER BY k.title COLLATE "C" LIMIT COALESCE($2::bigint,2147483647::bigint)
        "#).bind(access_user).bind(limit).fetch_all(&self.pool).await?)
    }

    pub async fn karaoke(&self, access_user: Option<&str>, id: &str) -> Result<Option<Value>> {
        Ok(sqlx::query_scalar::<_, Value>(r#"
            SELECT jsonb_build_object('id',k.id,'title',k.title,'artist',k.artist,
                'duration',k.duration,'format',k.format,'playCount',k.play_count)
            FROM karaokes k WHERE k.id=$2 AND ($1::text IS NULL OR EXISTS (
                SELECT 1 FROM category_karaokes ck JOIN user_categories uc ON uc.category_id=ck.category_id
                WHERE ck.karaoke_id=k.id AND uc.user_id=$1))
        "#).bind(access_user).bind(id).fetch_optional(&self.pool).await?)
    }

    pub async fn list_categories(&self, access_user: Option<&str>) -> Result<Vec<Value>> {
        Ok(sqlx::query_scalar::<_, Value>(r#"
            SELECT jsonb_build_object('id',c.id,'name',c.name,
                'songCount',(SELECT count(*) FROM category_songs cs WHERE cs.category_id=c.id),
                'karaokeCount',(SELECT count(*) FROM category_karaokes ck WHERE ck.category_id=c.id))
            FROM categories c WHERE ($1::text IS NULL OR EXISTS (
                SELECT 1 FROM user_categories uc WHERE uc.category_id=c.id AND uc.user_id=$1))
            ORDER BY c.name COLLATE "C"
        "#).bind(access_user).fetch_all(&self.pool).await?)
    }

    pub async fn list_albums(
        &self,
        viewer_id: &str,
        access_user: Option<&str>,
    ) -> Result<Vec<Value>> {
        Ok(sqlx::query_scalar::<_, Value>(r#"
          SELECT jsonb_build_object('id',a.id,'name',a.name,'artist',a.artist,'artistId',a.artist_id,
            'year',a.year,'songCount',count(DISTINCT s.id),'duration',COALESCE(sum(s.duration),0),
            'liked',EXISTS(SELECT 1 FROM user_likes ul WHERE ul.user_id=$1 AND ul.entity_type='album' AND ul.entity_id=a.id))
          FROM albums a LEFT JOIN songs s ON s.album_id=a.id
          WHERE ($2::text IS NULL OR EXISTS(SELECT 1 FROM songs s2 JOIN category_songs cs ON cs.song_id=s2.id
            JOIN user_categories uc ON uc.category_id=cs.category_id WHERE s2.album_id=a.id AND uc.user_id=$2))
          GROUP BY a.id ORDER BY a.name COLLATE "C",a.created_at
        "#).bind(viewer_id).bind(access_user).fetch_all(&self.pool).await?)
    }

    pub async fn album(
        &self,
        viewer_id: &str,
        access_user: Option<&str>,
        id: &str,
    ) -> Result<Option<Value>> {
        let Some(mut album) = sqlx::query_scalar::<_, Value>(r#"
          SELECT jsonb_build_object('id',a.id,'name',a.name,'artist',a.artist,'artistId',a.artist_id,
            'year',a.year,'songCount',count(DISTINCT s.id),'duration',COALESCE(sum(s.duration),0),
            'liked',EXISTS(SELECT 1 FROM user_likes ul WHERE ul.user_id=$1 AND ul.entity_type='album' AND ul.entity_id=a.id))
          FROM albums a LEFT JOIN songs s ON s.album_id=a.id
          WHERE a.id=$3 AND ($2::text IS NULL OR EXISTS(SELECT 1 FROM songs s2 JOIN category_songs cs ON cs.song_id=s2.id
            JOIN user_categories uc ON uc.category_id=cs.category_id WHERE s2.album_id=a.id AND uc.user_id=$2)) GROUP BY a.id
        "#).bind(viewer_id).bind(access_user).bind(id).fetch_optional(&self.pool).await? else { return Ok(None) };
        let songs = sqlx::query_scalar::<_, Value>(r#"
          SELECT jsonb_build_object('id',s.id,'title',s.title,'artist',s.artist,'artistId',s.artist_id,
            'album',s.album,'albumId',s.album_id,'year',s.year,'genre',s.genre,'duration',s.duration,
            'format',s.format,'bitrate',s.bitrate,'sampleRate',s.sample_rate,'trackNumber',s.track_number,
            'discNumber',s.disc_number,'playCount',s.play_count,
            'liked',EXISTS(SELECT 1 FROM user_likes ul WHERE ul.user_id=$1 AND ul.entity_type='song' AND ul.entity_id=s.id))
          FROM songs s WHERE s.album_id=$2 AND ($3::text IS NULL OR EXISTS(SELECT 1 FROM category_songs cs JOIN user_categories uc ON uc.category_id=cs.category_id WHERE cs.song_id=s.id AND uc.user_id=$3))
          ORDER BY s.disc_number,s.track_number,s.title COLLATE "C"
        "#).bind(viewer_id).bind(id).bind(access_user).fetch_all(&self.pool).await?;
        album["songs"] = Value::Array(songs);
        Ok(Some(album))
    }

    pub async fn list_artists(
        &self,
        viewer_id: &str,
        access_user: Option<&str>,
    ) -> Result<Vec<Value>> {
        Ok(sqlx::query_scalar::<_, Value>(r#"
          SELECT jsonb_build_object('id',a.id,'name',a.name,
            'albumCount',(SELECT count(*) FROM albums al WHERE al.artist_id=a.id),
            'songCount',(SELECT count(*) FROM songs s WHERE s.artist_id=a.id),
            'liked',EXISTS(SELECT 1 FROM user_likes ul WHERE ul.user_id=$1 AND ul.entity_type='artist' AND ul.entity_id=a.id))
          FROM artists a WHERE EXISTS(SELECT 1 FROM albums al WHERE al.artist_id=a.id)
          AND ($2::text IS NULL OR EXISTS(SELECT 1 FROM albums al JOIN songs s ON s.album_id=al.id
            JOIN category_songs cs ON cs.song_id=s.id JOIN user_categories uc ON uc.category_id=cs.category_id
            WHERE al.artist_id=a.id AND uc.user_id=$2)) ORDER BY a.name COLLATE "C"
        "#).bind(viewer_id).bind(access_user).fetch_all(&self.pool).await?)
    }

    pub async fn artist(
        &self,
        viewer_id: &str,
        access_user: Option<&str>,
        id: &str,
    ) -> Result<Option<Value>> {
        let Some(mut artist) = sqlx::query_scalar::<_, Value>(r#"
          SELECT jsonb_build_object('id',a.id,'name',a.name,
            'albumCount',(SELECT count(*) FROM albums al WHERE al.artist_id=a.id),
            'songCount',(SELECT count(*) FROM songs s WHERE s.artist_id=a.id),
            'liked',EXISTS(SELECT 1 FROM user_likes ul WHERE ul.user_id=$1 AND ul.entity_type='artist' AND ul.entity_id=a.id))
          FROM artists a WHERE a.id=$3 AND ($2::text IS NULL OR EXISTS(SELECT 1 FROM albums al JOIN songs s ON s.album_id=al.id
            JOIN category_songs cs ON cs.song_id=s.id JOIN user_categories uc ON uc.category_id=cs.category_id
            WHERE al.artist_id=a.id AND uc.user_id=$2))
        "#).bind(viewer_id).bind(access_user).bind(id).fetch_optional(&self.pool).await? else { return Ok(None) };
        let albums = sqlx::query_scalar::<_, Value>(r#"
          SELECT jsonb_build_object('id',a.id,'name',a.name,'artist',a.artist,'artistId',a.artist_id,
            'year',a.year,'songCount',count(DISTINCT s.id),'duration',COALESCE(sum(s.duration),0),
            'liked',EXISTS(SELECT 1 FROM user_likes ul WHERE ul.user_id=$1 AND ul.entity_type='album' AND ul.entity_id=a.id))
          FROM albums a LEFT JOIN songs s ON s.album_id=a.id WHERE a.artist_id=$2
          AND ($3::text IS NULL OR EXISTS(SELECT 1 FROM songs s2 JOIN category_songs cs ON cs.song_id=s2.id
            JOIN user_categories uc ON uc.category_id=cs.category_id WHERE s2.album_id=a.id AND uc.user_id=$3))
          GROUP BY a.id ORDER BY a.year,a.name
        "#).bind(viewer_id).bind(id).bind(access_user).fetch_all(&self.pool).await?;
        artist["albums"] = Value::Array(albums);
        Ok(Some(artist))
    }

    pub async fn category_detail(
        &self,
        access_user: Option<&str>,
        id: &str,
    ) -> Result<Option<Value>> {
        let Some(mut category) = sqlx::query_scalar::<_, Value>(
            r#"
            SELECT jsonb_build_object('id',c.id,'name',c.name,
              'songCount',(SELECT count(*) FROM category_songs cs WHERE cs.category_id=c.id),
              'karaokeCount',(SELECT count(*) FROM category_karaokes ck WHERE ck.category_id=c.id))
            FROM categories c WHERE c.id=$2 AND ($1::text IS NULL OR EXISTS (
              SELECT 1 FROM user_categories uc WHERE uc.category_id=c.id AND uc.user_id=$1))
        "#,
        )
        .bind(access_user)
        .bind(id)
        .fetch_optional(&self.pool)
        .await?
        else {
            return Ok(None);
        };
        let songs = sqlx::query_scalar::<_, Value>(r#"
          SELECT jsonb_build_object('id',s.id,'title',s.title,'artist',s.artist,'artistId',s.artist_id,
            'album',s.album,'albumId',s.album_id,'year',s.year,'genre',s.genre,'duration',s.duration,
            'format',s.format,'bitrate',s.bitrate,'sampleRate',s.sample_rate,'trackNumber',s.track_number,
            'discNumber',s.disc_number,'playCount',s.play_count)
          FROM category_songs cs JOIN songs s ON s.id=cs.song_id
          WHERE cs.category_id=$1 ORDER BY cs.position,s.title COLLATE "C"
        "#).bind(id).fetch_all(&self.pool).await?;
        let karaokes = sqlx::query_scalar::<_, Value>(r#"
          SELECT jsonb_build_object('id',k.id,'title',k.title,'artist',k.artist,'duration',k.duration,
            'format',k.format,'playCount',k.play_count)
          FROM category_karaokes ck JOIN karaokes k ON k.id=ck.karaoke_id
          WHERE ck.category_id=$1 ORDER BY ck.position,k.title COLLATE "C"
        "#).bind(id).fetch_all(&self.pool).await?;
        category["songs"] = Value::Array(songs);
        category["karaokes"] = Value::Array(karaokes);
        Ok(Some(category))
    }

    pub async fn set_like(&self, user_id: &str, kind: &str, id: &str, liked: bool) -> Result<()> {
        if !matches!(kind, "song" | "album" | "artist") {
            bail!("invalid like entity type");
        }
        if liked {
            sqlx::query("INSERT INTO user_likes(user_id,entity_type,entity_id,created_at) VALUES($1,$2,$3,now()) ON CONFLICT(user_id,entity_type,entity_id) DO NOTHING")
                .bind(user_id).bind(kind).bind(id).execute(&self.pool).await?;
        } else {
            sqlx::query(
                "DELETE FROM user_likes WHERE user_id=$1 AND entity_type=$2 AND entity_id=$3",
            )
            .bind(user_id)
            .bind(kind)
            .bind(id)
            .execute(&self.pool)
            .await?;
        }
        Ok(())
    }

    pub async fn liked_songs(
        &self,
        user_id: &str,
        access_user: Option<&str>,
    ) -> Result<Vec<Value>> {
        Ok(sqlx::query_scalar::<_, Value>(r#"
          SELECT jsonb_build_object('id',s.id,'title',s.title,'artist',s.artist,'artistId',s.artist_id,
            'album',s.album,'albumId',s.album_id,'year',s.year,'genre',s.genre,'duration',s.duration,
            'format',s.format,'playCount',s.play_count,'liked',true)
          FROM user_likes ul JOIN songs s ON s.id=ul.entity_id
          WHERE ul.user_id=$1 AND ul.entity_type='song'
            AND ($2::text IS NULL OR EXISTS(SELECT 1 FROM category_songs cs JOIN user_categories uc ON uc.category_id=cs.category_id WHERE cs.song_id=s.id AND uc.user_id=$2))
          ORDER BY ul.created_at DESC,s.title COLLATE "C"
        "#).bind(user_id).bind(access_user).fetch_all(&self.pool).await?)
    }

    pub async fn history(
        &self,
        user_id: &str,
        access_user: Option<&str>,
        limit: i64,
    ) -> Result<Vec<Value>> {
        Ok(sqlx::query_scalar::<_, Value>(r#"
          SELECT jsonb_build_object('id',s.id,'title',s.title,'artist',s.artist,'artistId',s.artist_id,
            'album',s.album,'albumId',s.album_id,'year',s.year,'genre',s.genre,'duration',s.duration,
            'format',s.format,'playCount',s.play_count,'playedAt',h.played_at)
          FROM history h JOIN songs s ON s.id=h.song_id
          WHERE h.user_id=$1 AND ($2::text IS NULL OR EXISTS(SELECT 1 FROM category_songs cs JOIN user_categories uc ON uc.category_id=cs.category_id WHERE cs.song_id=s.id AND uc.user_id=$2))
          ORDER BY h.played_at DESC LIMIT $3
        "#).bind(user_id).bind(access_user).bind(limit).fetch_all(&self.pool).await?)
    }

    pub async fn register_play(
        &self,
        user_id: &str,
        access_user: Option<&str>,
        song_id: &str,
    ) -> Result<bool> {
        let mut tx = self.pool.begin().await?;
        let updated = sqlx::query("UPDATE songs s SET play_count=play_count+1,last_played_at=now() WHERE id=$1 AND ($2::text IS NULL OR EXISTS(SELECT 1 FROM category_songs cs JOIN user_categories uc ON uc.category_id=cs.category_id WHERE cs.song_id=s.id AND uc.user_id=$2))")
            .bind(song_id).bind(access_user).execute(&mut *tx).await?;
        if updated.rows_affected() == 0 {
            tx.rollback().await?;
            return Ok(false);
        }
        sqlx::query("INSERT INTO history(song_id,user_id,played_at) VALUES($1,$2,now())")
            .bind(song_id)
            .bind(user_id)
            .execute(&mut *tx)
            .await?;
        tx.commit().await?;
        Ok(true)
    }

    pub async fn get_queue(&self, user_id: &str) -> Result<Value> {
        let row = sqlx::query("SELECT data FROM play_queue WHERE user_id=$1")
            .bind(user_id)
            .fetch_optional(&self.pool)
            .await?;
        Ok(row
            .map(|r| r.get::<Value, _>("data"))
            .unwrap_or_else(|| Value::Array(vec![])))
    }

    pub async fn save_queue(&self, user_id: &str, data: Value) -> Result<()> {
        let data = if data.is_array() {
            data
        } else {
            Value::Array(vec![])
        };
        sqlx::query("INSERT INTO play_queue(user_id,data,updated_at) VALUES($1,$2,now()) ON CONFLICT(user_id) DO UPDATE SET data=EXCLUDED.data,updated_at=now()")
            .bind(user_id).bind(data).execute(&self.pool).await?;
        Ok(())
    }

    pub async fn can_access_song(&self, access_user: Option<&str>, id: &str) -> Result<bool> {
        Ok(sqlx::query_scalar("SELECT EXISTS(SELECT 1 FROM songs s WHERE s.id=$2 AND ($1::text IS NULL OR EXISTS(SELECT 1 FROM category_songs cs JOIN user_categories uc ON uc.category_id=cs.category_id WHERE cs.song_id=s.id AND uc.user_id=$1)))")
            .bind(access_user).bind(id).fetch_one(&self.pool).await?)
    }

    pub async fn can_access_karaoke(&self, access_user: Option<&str>, id: &str) -> Result<bool> {
        Ok(sqlx::query_scalar("SELECT EXISTS(SELECT 1 FROM karaokes k WHERE k.id=$2 AND ($1::text IS NULL OR EXISTS(SELECT 1 FROM category_karaokes ck JOIN user_categories uc ON uc.category_id=ck.category_id WHERE ck.karaoke_id=k.id AND uc.user_id=$1)))")
            .bind(access_user).bind(id).fetch_one(&self.pool).await?)
    }

    pub async fn playlists(&self, user_id: &str, access_user: Option<&str>) -> Result<Vec<Value>> {
        Ok(sqlx::query_scalar::<_, Value>(
            r#"
          SELECT jsonb_build_object('id',p.id,'name',p.name,'comment',p.comment,'owner',p.owner,
            'songCount',count(s.id),'duration',COALESCE(sum(s.duration),0))
          FROM playlists p LEFT JOIN playlist_tracks pt ON pt.playlist_id=p.id
          LEFT JOIN songs s ON s.id=pt.song_id AND ($2::text IS NULL OR EXISTS(
            SELECT 1 FROM category_songs cs JOIN user_categories uc ON uc.category_id=cs.category_id
            WHERE cs.song_id=s.id AND uc.user_id=$2))
          WHERE p.user_id=$1 GROUP BY p.id ORDER BY p.updated_at DESC,p.name COLLATE "C"
        "#,
        )
        .bind(user_id)
        .bind(access_user)
        .fetch_all(&self.pool)
        .await?)
    }

    pub async fn playlist(
        &self,
        user_id: &str,
        access_user: Option<&str>,
        id: &str,
    ) -> Result<Option<Value>> {
        let Some(mut playlist) = sqlx::query_scalar::<_, Value>(
            r#"
          SELECT jsonb_build_object('id',p.id,'name',p.name,'comment',p.comment,'owner',p.owner,
            'songCount',count(s.id),'duration',COALESCE(sum(s.duration),0))
          FROM playlists p LEFT JOIN playlist_tracks pt ON pt.playlist_id=p.id
          LEFT JOIN songs s ON s.id=pt.song_id AND ($2::text IS NULL OR EXISTS(
            SELECT 1 FROM category_songs cs JOIN user_categories uc ON uc.category_id=cs.category_id
            WHERE cs.song_id=s.id AND uc.user_id=$2))
          WHERE p.user_id=$1 AND p.id=$3 GROUP BY p.id
        "#,
        )
        .bind(user_id)
        .bind(access_user)
        .bind(id)
        .fetch_optional(&self.pool)
        .await?
        else {
            return Ok(None);
        };
        let tracks = sqlx::query_scalar::<_, Value>(r#"
          SELECT jsonb_build_object('entryId',pt.id,'song',jsonb_build_object('id',s.id,'title',s.title,
            'artist',s.artist,'artistId',s.artist_id,'album',s.album,'albumId',s.album_id,'year',s.year,
            'genre',s.genre,'duration',s.duration,'format',s.format,'playCount',s.play_count,
            'liked',EXISTS(SELECT 1 FROM user_likes ul WHERE ul.user_id=$1 AND ul.entity_type='song' AND ul.entity_id=s.id)))
          FROM playlist_tracks pt JOIN songs s ON s.id=pt.song_id
          WHERE pt.playlist_id=$2 AND ($3::text IS NULL OR EXISTS(SELECT 1 FROM category_songs cs JOIN user_categories uc ON uc.category_id=cs.category_id WHERE cs.song_id=s.id AND uc.user_id=$3))
          ORDER BY pt.position
        "#).bind(user_id).bind(id).bind(access_user).fetch_all(&self.pool).await?;
        playlist["songs"] = Value::Array(tracks);
        Ok(Some(playlist))
    }

    pub async fn create_playlist(
        &self,
        user_id: &str,
        name: &str,
        comment: &str,
        song_ids: &[String],
        access_user: Option<&str>,
    ) -> Result<String> {
        let id = new_id();
        let mut tx = self.pool.begin().await?;
        sqlx::query("INSERT INTO playlists(id,name,comment,owner,user_id,created_at,updated_at) VALUES($1,$2,$3,$4,$4,now(),now())")
            .bind(&id).bind(name).bind(comment).bind(user_id).execute(&mut *tx).await?;
        append_tracks(&mut tx, &id, song_ids, access_user).await?;
        tx.commit().await?;
        Ok(id)
    }

    pub async fn update_playlist(
        &self,
        user_id: &str,
        id: &str,
        name: Option<&str>,
        comment: Option<&str>,
    ) -> Result<bool> {
        let result = sqlx::query("UPDATE playlists SET name=COALESCE($3,name),comment=COALESCE($4,comment),updated_at=now() WHERE user_id=$1 AND id=$2")
            .bind(user_id).bind(id).bind(name).bind(comment).execute(&self.pool).await?;
        Ok(result.rows_affected() > 0)
    }

    pub async fn delete_playlist(&self, user_id: &str, id: &str) -> Result<bool> {
        let result = sqlx::query("DELETE FROM playlists WHERE user_id=$1 AND id=$2")
            .bind(user_id)
            .bind(id)
            .execute(&self.pool)
            .await?;
        Ok(result.rows_affected() > 0)
    }

    pub async fn add_playlist_tracks(
        &self,
        user_id: &str,
        id: &str,
        song_ids: &[String],
        access_user: Option<&str>,
    ) -> Result<bool> {
        if !sqlx::query_scalar::<_, bool>(
            "SELECT EXISTS(SELECT 1 FROM playlists WHERE user_id=$1 AND id=$2)",
        )
        .bind(user_id)
        .bind(id)
        .fetch_one(&self.pool)
        .await?
        {
            return Ok(false);
        }
        let mut tx = self.pool.begin().await?;
        append_tracks(&mut tx, id, song_ids, access_user).await?;
        tx.commit().await?;
        Ok(true)
    }

    pub async fn remove_playlist_track(
        &self,
        user_id: &str,
        id: &str,
        entry_id: &str,
    ) -> Result<bool> {
        let mut tx = self.pool.begin().await?;
        let deleted = sqlx::query("DELETE FROM playlist_tracks pt USING playlists p WHERE pt.playlist_id=p.id AND p.user_id=$1 AND p.id=$2 AND pt.id=$3")
            .bind(user_id).bind(id).bind(entry_id).execute(&mut *tx).await?;
        if deleted.rows_affected() == 0 {
            tx.rollback().await?;
            return Ok(false);
        }
        renumber(&mut tx, id).await?;
        tx.commit().await?;
        Ok(true)
    }

    pub async fn reorder_playlist(
        &self,
        user_id: &str,
        id: &str,
        from: usize,
        to: usize,
    ) -> Result<bool> {
        let mut tx = self.pool.begin().await?;
        if !sqlx::query_scalar::<_, bool>(
            "SELECT EXISTS(SELECT 1 FROM playlists WHERE user_id=$1 AND id=$2)",
        )
        .bind(user_id)
        .bind(id)
        .fetch_one(&mut *tx)
        .await?
        {
            tx.rollback().await?;
            return Ok(false);
        }
        let rows =
            sqlx::query("SELECT id FROM playlist_tracks WHERE playlist_id=$1 ORDER BY position")
                .bind(id)
                .fetch_all(&mut *tx)
                .await?;
        if from >= rows.len() || to >= rows.len() {
            tx.rollback().await?;
            return Ok(false);
        }
        let mut ids = rows
            .iter()
            .map(|r| r.get::<String, _>("id"))
            .collect::<Vec<_>>();
        let item = ids.remove(from);
        ids.insert(to, item);
        for (position, entry) in ids.iter().enumerate() {
            sqlx::query("UPDATE playlist_tracks SET position=$2 WHERE id=$1")
                .bind(entry)
                .bind(position as i32)
                .execute(&mut *tx)
                .await?;
        }
        tx.commit().await?;
        Ok(true)
    }
}

async fn append_tracks(
    tx: &mut sqlx::Transaction<'_, sqlx::Postgres>,
    playlist: &str,
    songs: &[String],
    access_user: Option<&str>,
) -> Result<()> {
    if songs.is_empty() {
        return Ok(());
    }
    let permitted: i64 = sqlx::query_scalar("SELECT count(*) FROM songs s WHERE s.id=ANY($1) AND ($2::text IS NULL OR EXISTS(SELECT 1 FROM category_songs cs JOIN user_categories uc ON uc.category_id=cs.category_id WHERE cs.song_id=s.id AND uc.user_id=$2))")
        .bind(songs).bind(access_user).fetch_one(&mut **tx).await?;
    if permitted != songs.len() as i64 {
        bail!("one or more tracks are unavailable");
    }
    let start: i64 = sqlx::query_scalar(
        "SELECT COALESCE(max(position)+1,0) FROM playlist_tracks WHERE playlist_id=$1",
    )
    .bind(playlist)
    .fetch_one(&mut **tx)
    .await?;
    for (offset, song) in songs.iter().enumerate() {
        sqlx::query(
            "INSERT INTO playlist_tracks(id,playlist_id,song_id,position) VALUES($1,$2,$3,$4)",
        )
        .bind(new_id())
        .bind(playlist)
        .bind(song)
        .bind(start + offset as i64)
        .execute(&mut **tx)
        .await?;
    }
    Ok(())
}

async fn renumber(tx: &mut sqlx::Transaction<'_, sqlx::Postgres>, playlist: &str) -> Result<()> {
    let rows = sqlx::query("SELECT id FROM playlist_tracks WHERE playlist_id=$1 ORDER BY position")
        .bind(playlist)
        .fetch_all(&mut **tx)
        .await?;
    for (position, row) in rows.iter().enumerate() {
        sqlx::query("UPDATE playlist_tracks SET position=$2 WHERE id=$1")
            .bind(row.get::<String, _>("id"))
            .bind(position as i32)
            .execute(&mut **tx)
            .await?;
    }
    Ok(())
}

fn new_id() -> String {
    let mut bytes = [0_u8; 16];
    use rand::RngCore;
    rand::rngs::OsRng.fill_bytes(&mut bytes);
    hex::encode(bytes)
}

fn like_pattern(query: &str) -> String {
    let mut pattern = String::with_capacity(query.len() + 2);
    pattern.push('%');
    for ch in query.chars() {
        if matches!(ch, '\\' | '%' | '_') {
            pattern.push('\\');
        }
        pattern.push(ch);
    }
    pattern.push('%');
    pattern
}
