package store

import (
	"context"
	"math"
	"sort"
	"strings"
	"time"

	"play-music/internal/model"
)

type RecommendedArtist struct {
	Name   string `json:"name"`
	SongID string `json:"songId"`
}
type Recommendations struct {
	Songs        []model.Song        `json:"songs"`
	Artists      []RecommendedArtist `json:"artists"`
	Tastes       []string            `json:"tastes"`
	Personalized bool                `json:"personalized"`
}
type listenSignal struct {
	SongID string
	At     time.Time
}

func (s *Store) RegisterQualifiedListen(ctx context.Context, userID, songID, playID string) error {
	_, err := s.pool.Exec(ctx, `INSERT INTO qualified_listens(user_id,song_id,play_id) VALUES($1,$2,$3) ON CONFLICT(user_id,play_id) DO NOTHING`, userID, songID, playID)
	return err
}

func (s *Store) Recommendations(ctx context.Context, userID string) (Recommendations, error) {
	songs, err := s.AllSongs(ctx, "")
	if err != nil {
		return Recommendations{}, err
	}
	categories := map[string][]string{}
	rows, err := s.pool.Query(ctx, `SELECT cs.song_id,c.name FROM category_songs cs JOIN categories c ON c.id=cs.category_id`)
	if err != nil {
		return Recommendations{}, err
	}
	for rows.Next() {
		var id, name string
		if err = rows.Scan(&id, &name); err != nil {
			rows.Close()
			return Recommendations{}, err
		}
		categories[id] = append(categories[id], name)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return Recommendations{}, err
	}
	rows, err = s.pool.Query(ctx, `SELECT song_id,listened_at FROM qualified_listens WHERE user_id=$1 AND listened_at >= now()-interval '90 days' ORDER BY listened_at DESC LIMIT 5000`, userID)
	if err != nil {
		return Recommendations{}, err
	}
	var plays []listenSignal
	for rows.Next() {
		var p listenSignal
		if err = rows.Scan(&p.SongID, &p.At); err != nil {
			rows.Close()
			return Recommendations{}, err
		}
		plays = append(plays, p)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return Recommendations{}, err
	}
	rows, err = s.pool.Query(ctx, `SELECT entity_id,created_at FROM user_likes WHERE user_id=$1 AND entity_type='song'`, userID)
	if err != nil {
		return Recommendations{}, err
	}
	likes := map[string]time.Time{}
	for rows.Next() {
		var id string
		var likedAt time.Time
		if err = rows.Scan(&id, &likedAt); err != nil {
			rows.Close()
			return Recommendations{}, err
		}
		likes[id] = likedAt
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return Recommendations{}, err
	}
	return rankRecommendations(songs, categories, plays, likes, time.Now()), nil
}

func rankRecommendations(songs []model.Song, categories map[string][]string, plays []listenSignal, likes map[string]time.Time, now time.Time) Recommendations {
	result := Recommendations{Songs: []model.Song{}, Artists: []RecommendedArtist{}, Tastes: []string{}}
	byID := map[string]model.Song{}
	for _, song := range songs {
		byID[song.ID] = song
	}
	artistWeight, genreWeight, categoryWeight := map[string]float64{}, map[string]float64{}, map[string]float64{}
	recent := map[string]bool{}
	add := func(song model.Song, weight float64) {
		if key := recommendationArtistKey(song); key != "" {
			artistWeight[key] += weight
		}
		if g := strings.TrimSpace(song.Genre); g != "" {
			genreWeight[g] += weight
		}
		for _, c := range categories[song.ID] {
			categoryWeight[c] += weight
		}
	}
	for _, p := range plays {
		song, ok := byID[p.SongID]
		if !ok {
			continue
		}
		days := math.Max(0, now.Sub(p.At).Hours()/24)
		if days > 90 {
			continue
		}
		add(song, 1/(1+days/30))
		if days < 7 {
			recent[p.SongID] = true
		}
		result.Personalized = true
	}
	for id, likedAt := range likes {
		if song, ok := byID[id]; ok {
			days := math.Max(0, now.Sub(likedAt).Hours()/24)
			add(song, 3/(1+days/90))
			result.Personalized = true
		}
	}
	type ranked struct {
		song  model.Song
		score float64
	}
	rankedSongs := make([]ranked, 0, len(songs))
	for _, song := range songs {
		_, song.Liked = likes[song.ID]
		score := 3*artistWeight[recommendationArtistKey(song)] + 2*genreWeight[strings.TrimSpace(song.Genre)]
		for _, c := range categories[song.ID] {
			score += categoryWeight[c]
		}
		if recent[song.ID] {
			score *= 0.35
		}
		score += math.Log1p(float64(song.PlayCount)) * 0.05
		rankedSongs = append(rankedSongs, ranked{song, score})
	}
	sort.SliceStable(rankedSongs, func(i, j int) bool {
		if rankedSongs[i].score != rankedSongs[j].score {
			return rankedSongs[i].score > rankedSongs[j].score
		}
		if !rankedSongs[i].song.CreatedAt.Equal(rankedSongs[j].song.CreatedAt) {
			return rankedSongs[i].song.CreatedAt.After(rankedSongs[j].song.CreatedAt)
		}
		return rankedSongs[i].song.ID < rankedSongs[j].song.ID
	})
	counts := map[string]int{}
	included := map[string]bool{}
	for _, r := range rankedSongs {
		key := recommendationArtistKey(r.song)
		if key == "" {
			key = "song:" + r.song.ID
		}
		if counts[key] >= 3 {
			continue
		}
		result.Songs = append(result.Songs, r.song)
		included[r.song.ID] = true
		counts[key]++
		if len(result.Songs) == 30 {
			break
		}
	}
	for _, r := range rankedSongs {
		if len(result.Songs) == 30 {
			break
		}
		if !included[r.song.ID] {
			result.Songs = append(result.Songs, r.song)
		}
	}
	seenArtists := map[string]bool{}
	for _, r := range rankedSongs {
		name := strings.TrimSpace(r.song.Artist)
		key := strings.ToLower(name)
		if name != "" && key != "desconhecido" && !seenArtists[key] {
			result.Artists = append(result.Artists, RecommendedArtist{Name: name, SongID: r.song.ID})
			seenArtists[key] = true
			if len(result.Artists) == 10 {
				break
			}
		}
	}

	tastes := map[string]float64{}
	for k, v := range genreWeight {
		tastes[k] += v
	}
	for k, v := range categoryWeight {
		tastes[k] += v
	}
	for k := range tastes {
		result.Tastes = append(result.Tastes, k)
	}
	sort.Slice(result.Tastes, func(i, j int) bool {
		if tastes[result.Tastes[i]] != tastes[result.Tastes[j]] {
			return tastes[result.Tastes[i]] > tastes[result.Tastes[j]]
		}
		return result.Tastes[i] < result.Tastes[j]
	})
	if len(result.Tastes) > 8 {
		result.Tastes = result.Tastes[:8]
	}
	return result
}

func recommendationArtistKey(song model.Song) string {
	name := strings.ToLower(strings.TrimSpace(song.Artist))
	if name == "desconhecido" {
		return ""
	}
	if name != "" {
		return "name:" + name
	}
	if song.ArtistID != "" {
		return "id:" + song.ArtistID
	}
	return ""
}
