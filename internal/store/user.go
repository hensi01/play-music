package store

import (
	"context"
	"encoding/json"
)

// ---------- likes ----------

func (s *Store) SetLike(ctx context.Context, entityType, entityID string, liked bool) error {
	if liked {
		_, err := s.pool.Exec(ctx,
			`INSERT INTO user_likes (entity_type, entity_id, created_at) VALUES ($1, $2, now())
			 ON CONFLICT (entity_type, entity_id) DO NOTHING`, entityType, entityID)
		return err
	}
	_, err := s.pool.Exec(ctx,
		"DELETE FROM user_likes WHERE entity_type=$1 AND entity_id=$2", entityType, entityID)
	return err
}

// ---------- history ----------

func (s *Store) HistoryCount(ctx context.Context) (int64, error) {
	var n int64
	err := s.pool.QueryRow(ctx, "SELECT count(*) FROM history").Scan(&n)
	return n, err
}

// ---------- play queue (single-row JSONB) ----------

var emptyQueue = []byte("[]")

func (s *Store) GetPlayQueue(ctx context.Context) ([]byte, error) {
	var data []byte
	err := s.pool.QueryRow(ctx, "SELECT data FROM play_queue WHERE id=1").Scan(&data)
	if err != nil {
		if err.Error() == "no rows in result set" {
			return emptyQueue, nil
		}
		return nil, err
	}
	return data, nil
}

func (s *Store) SavePlayQueue(ctx context.Context, data []byte) error {
	if len(data) == 0 {
		data = emptyQueue
	}
	if !json.Valid(data) {
		data = emptyQueue
	}
	_, err := s.pool.Exec(ctx,
		`INSERT INTO play_queue (id, data, updated_at) VALUES (1, $1, now())
		 ON CONFLICT (id) DO UPDATE SET data=EXCLUDED.data, updated_at=now()`, data)
	return err
}
