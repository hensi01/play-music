package api

import (
	"testing"
	"time"

	"github.com/hensi01/play-music/model"
)

func TestDetectImageType(t *testing.T) {
	cases := []struct {
		name string
		data []byte
		want string
	}{
		{"webp", append([]byte("RIFF\x00\x00\x00\x00WEBPVP8 "), make([]byte, 16)...), "image/webp"},
		{"png", []byte("\x89PNG\r\n\x1a\npayload"), "image/png"},
		{"jpeg", []byte("\xff\xd8\xff\xe0payload"), "image/jpeg"},
		{"unknown", []byte("hello world, not an image"), "text/plain; charset=utf-8"},
	}
	for _, c := range cases {
		if got := detectImageType(c.data); got != c.want {
			t.Errorf("%s: detectImageType() = %q, want %q", c.name, got, c.want)
		}
	}
}

func TestToSong(t *testing.T) {
	now := time.Now()
	mf := &model.MediaFile{
		ID:          "song1",
		Title:       "Test Song",
		AlbumID:     "album1",
		Album:       "Test Album",
		ArtistID:    "artist1",
		Artist:      "Test Artist",
		AlbumArtist: "Test Artist",
		Duration:    120.5,
		TrackNumber: 3,
		DiscNumber:  1,
		Year:        2024,
		Genre:       "Rock",
		Suffix:      "flac",
		BitRate:     900,
		Annotations: model.Annotations{
			PlayCount: 5,
			PlayDate:  &now,
			Starred:   true,
		},
	}
	s := toSong(mf)
	if s.ID != "song1" || s.Title != "Test Song" || s.Format != "flac" || s.PlayCount != 5 || !s.Liked {
		t.Fatalf("toSong() mismatch: %+v", s)
	}
	if s.LastPlayedAt == nil || !s.LastPlayedAt.Equal(now) {
		t.Fatalf("toSong() lastPlayedAt = %v, want %v", s.LastPlayedAt, now)
	}
}

func TestToAlbumUsesMaxYearAndFirstGenre(t *testing.T) {
	al := &model.Album{
		ID:            "album1",
		Name:          "Test Album",
		AlbumArtistID: "artist1",
		AlbumArtist:   "Test Artist",
		MinYear:       1999,
		MaxYear:       2024,
		SongCount:     10,
		Duration:      600,
		Genres:        model.Genres{{Name: "Rock"}, {Name: "Pop"}},
		Annotations:   model.Annotations{Starred: true, PlayCount: 3},
	}
	a := toAlbum(al)
	if a.Year != 2024 {
		t.Fatalf("toAlbum() year = %d, want 2024 (max_year)", a.Year)
	}
	if a.Genre != "Rock" {
		t.Fatalf("toAlbum() genre = %q, want %q", a.Genre, "Rock")
	}
	if a.SongCount != 10 || a.PlayCount != 3 || !a.Liked {
		t.Fatalf("toAlbum() mismatch: %+v", a)
	}
}
