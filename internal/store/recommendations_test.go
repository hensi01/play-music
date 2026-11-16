package store

import (
	"play-music/internal/model"
	"testing"
	"time"
)

func TestRecommendationsPersonalizeAndRespectIndividualLikes(t *testing.T) {
	now := time.Date(2026, 11, 6, 0, 0, 0, 0, time.UTC)
	songs := []model.Song{
		{ID: "a", ArtistID: "artist-a", Artist: "A", Genre: "rock", CreatedAt: now.Add(-time.Hour)},
		{ID: "b", ArtistID: "artist-a", Artist: "A", Genre: "rock", CreatedAt: now.Add(-2 * time.Hour)},
		{ID: "c", ArtistID: "artist-c", Artist: "C", Genre: "jazz", CreatedAt: now.Add(-3 * time.Hour)},
		{ID: "d", ArtistID: "artist-d", Artist: "D", Genre: "rock", CreatedAt: now.Add(-4 * time.Hour)},
	}
	categories := map[string][]string{"a": {"Rock"}, "b": {"Rock"}, "d": {"Rock"}, "c": {"Jazz"}}
	first := rankRecommendations(songs, categories, []listenSignal{{SongID: "a", At: now.Add(-time.Hour)}}, map[string]time.Time{"a": now}, now)
	if !first.Personalized || len(first.Songs) != 4 || first.Songs[0].ID != "b" || len(first.Artists) < 2 {
		t.Fatalf("unexpected personalized ranking: %+v", first)
	}
	for _, s := range first.Songs {
		if s.Liked != (s.ID == "a") {
			t.Fatalf("wrong user like for %s", s.ID)
		}
	}
	second := rankRecommendations(songs, categories, nil, map[string]time.Time{"c": now}, now)
	if second.Songs[0].ID != "c" || second.Songs[0].Liked != true {
		t.Fatalf("second user's preference ignored: %+v", second.Songs)
	}
}
func TestRecommendationsFallbackAndOldHistory(t *testing.T) {
	now := time.Now()
	songs := []model.Song{{ID: "older", CreatedAt: now.Add(-time.Hour)}, {ID: "newer", CreatedAt: now}}
	got := rankRecommendations(songs, nil, []listenSignal{{SongID: "older", At: now.Add(-91 * 24 * time.Hour)}}, nil, now)
	if !got.Personalized || len(got.Songs) != 2 || got.Songs[0].ID != "older" {
		t.Fatalf("old history should still contribute: %+v", got)
	}
	empty := rankRecommendations(nil, nil, nil, nil, now)
	if empty.Songs == nil || empty.Artists == nil || empty.Tastes == nil {
		t.Fatal("empty lists must be JSON arrays")
	}
}

func TestRecommendationsUseAlbumAndNormalizeGenres(t *testing.T) {
	now := time.Now()
	songs := []model.Song{
		{ID: "heard", Artist: "Band", Album: "First", Genre: " Rock "},
		{ID: "album", Artist: "Band", Album: "First", Genre: "rock"},
		{ID: "other", Artist: "Band", Album: "Second", Genre: "ROCK"},
		{ID: "jazz", Artist: "Other", Album: "Jazz", Genre: "jazz"},
	}
	got := rankRecommendations(songs, nil, []listenSignal{{SongID: "heard", At: now}}, nil, now)
	if got.Songs[0].ID != "album" || got.Songs[len(got.Songs)-1].ID != "jazz" {
		t.Fatalf("album and genre affinity missing: %+v", got.Songs)
	}
}

func TestRecommendationsRecentHistoryAndRepeatedPlays(t *testing.T) {
	now := time.Now()
	songs := []model.Song{{ID: "old", Artist: "Old"}, {ID: "new", Artist: "New"}, {ID: "old-match", Artist: "Old"}, {ID: "new-match", Artist: "New"}}
	plays := []listenSignal{{SongID: "old", At: now.Add(-365 * 24 * time.Hour)}}
	for i := 0; i < 10; i++ {
		plays = append(plays, listenSignal{SongID: "new", At: now.Add(-time.Hour)})
	}
	got := rankRecommendations(songs, nil, plays, nil, now)
	if got.Songs[0].ID != "new-match" {
		t.Fatalf("recent listening preference not reflected: %+v", got.Songs)
	}
}

func TestRecommendationsHandleUnknownMetadataAndMissingSongs(t *testing.T) {
	now := time.Now()
	songs := []model.Song{{ID: "heard"}, {ID: "other"}}
	got := rankRecommendations(songs, nil, []listenSignal{{SongID: "deleted", At: now}, {SongID: "heard", At: now.Add(-8 * 24 * time.Hour)}}, nil, now)
	if !got.Personalized || got.Songs[0].ID != "heard" {
		t.Fatalf("metadata-free song should use personal history: %+v", got)
	}
}
func TestRecommendationsLimitArtistRepetition(t *testing.T) {
	now := time.Now()
	songs := []model.Song{{ID: "a1", ArtistID: "a"}, {ID: "a2", ArtistID: "a"}, {ID: "a3", ArtistID: "a"}, {ID: "a4", ArtistID: "a"}, {ID: "b1", ArtistID: "b"}}
	got := rankRecommendations(songs, nil, nil, map[string]time.Time{"a1": now}, now)
	if len(got.Songs) != 5 || got.Songs[3].ID != "b1" {
		t.Fatalf("insufficient artist diversity: %+v", got.Songs)
	}
}

func TestRecentLikesWeighMoreThanOldLikes(t *testing.T) {
	now := time.Now()
	songs := []model.Song{{ID: "old", ArtistID: "old-artist"}, {ID: "new", ArtistID: "new-artist"}}
	got := rankRecommendations(songs, nil, nil, map[string]time.Time{"old": now.Add(-365 * 24 * time.Hour), "new": now}, now)
	if got.Songs[0].ID != "new" {
		t.Fatalf("recent like did not receive higher weight: %+v", got.Songs)
	}
}

func TestRecommendationsSeparateArtistsWithSharedLegacyID(t *testing.T) {
	now := time.Now()
	songs := []model.Song{{ID: "a", ArtistID: "shared", Artist: "Band A"}, {ID: "b", ArtistID: "shared", Artist: "Band B"}}
	got := rankRecommendations(songs, nil, nil, map[string]time.Time{"a": now}, now)
	if got.Songs[0].ID != "a" || len(got.Artists) != 2 || got.Artists[0].Name != "Band A" || got.Artists[1].Name != "Band B" {
		t.Fatalf("artists incorrectly merged: %+v", got)
	}
}
