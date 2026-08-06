package store

import (
	"context"
	"encoding/json"
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
