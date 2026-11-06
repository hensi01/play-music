package store

import (
	"context"
	"encoding/json"

	"play-music/internal/model"
)

// ---------- likes ----------

func (s *Store) SetLike(ctx context.Context, userID, entityType, entityID string, liked bool) error {
	if liked {
		_, err := s.pool.Exec(ctx,
			`INSERT INTO user_likes (user_id, entity_type, entity_id, created_at) VALUES ($1, $2, $3, now())
			 ON CONFLICT (user_id, entity_type, entity_id) DO NOTHING`, userID, entityType, entityID)
		return err
	}
	_, err := s.pool.Exec(ctx,
		"DELETE FROM user_likes WHERE user_id=$1 AND entity_type=$2 AND entity_id=$3",
		userID, entityType, entityID)
	return err
}

// ---------- play queue (per-user JSONB row) ----------

var emptyQueue = []byte("[]")

func (s *Store) GetPlayQueue(ctx context.Context, userID string) ([]byte, error) {
	var data []byte
	err := s.pool.QueryRow(ctx, "SELECT data FROM play_queue WHERE user_id=$1", userID).Scan(&data)
	if err != nil {
		if err.Error() == "no rows in result set" {
			return emptyQueue, nil
		}
		return nil, err
	}
	return data, nil
}

func (s *Store) SavePlayQueue(ctx context.Context, userID string, data []byte) error {
	if len(data) == 0 {
		data = emptyQueue
	}
	if !json.Valid(data) {
		data = emptyQueue
	}
	_, err := s.pool.Exec(ctx,
		`INSERT INTO play_queue (user_id, data, updated_at) VALUES ($1, $2, now())
		 ON CONFLICT (user_id) DO UPDATE SET data=EXCLUDED.data, updated_at=now()`, userID, data)
	return err
}

// ApplySongLikes decorates songs exclusively with the requesting user's likes.
func (s *Store) ApplySongLikes(ctx context.Context, userID string, songs []model.Song) error {
	if len(songs) == 0 {
		return nil
	}
	ids := make([]string, len(songs))
	for i := range songs {
		ids[i] = songs[i].ID
		songs[i].Liked = false
	}
	rows, err := s.pool.Query(ctx, `SELECT entity_id FROM user_likes WHERE user_id=$1 AND entity_type='song' AND entity_id=ANY($2)`, userID, ids)
	if err != nil {
		return err
	}
	defer rows.Close()
	likes := map[string]bool{}
	for rows.Next() {
		var id string
		if err = rows.Scan(&id); err != nil {
			return err
		}
		likes[id] = true
	}
	if err = rows.Err(); err != nil {
		return err
	}
	for i := range songs {
		songs[i].Liked = likes[songs[i].ID]
	}
	return nil
}
