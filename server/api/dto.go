package api

import (
	"time"

	"github.com/hensi01/play-music/model"
)

// Song is the wire representation of a media file.
type Song struct {
	ID           string     `json:"id"`
	Title        string     `json:"title"`
	AlbumID      string     `json:"albumId"`
	Album        string     `json:"album"`
	ArtistID     string     `json:"artistId"`
	Artist       string     `json:"artist"`
	AlbumArtist  string     `json:"albumArtist"`
	Duration     float32    `json:"duration"`
	TrackNumber  int        `json:"trackNumber"`
	DiscNumber   int        `json:"discNumber"`
	Year         int        `json:"year"`
	Genre        string     `json:"genre,omitempty"`
	Format       string     `json:"format"`
	BitRate      int        `json:"bitRate"`
	PlayCount    int64      `json:"playCount"`
	Liked        bool       `json:"liked"`
	LastPlayedAt *time.Time `json:"lastPlayedAt,omitempty"`
}

// Album is the wire representation of an album.
type Album struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	ArtistID  string    `json:"artistId"`
	Artist    string    `json:"artist"`
	Year      int       `json:"year"`
	Genre     string    `json:"genre,omitempty"`
	SongCount int       `json:"songCount"`
	Duration  float32   `json:"duration"`
	PlayCount int64     `json:"playCount"`
	Liked     bool      `json:"liked"`
	CreatedAt time.Time `json:"createdAt"`
}

// Artist is the wire representation of an artist.
type Artist struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	AlbumCount int    `json:"albumCount"`
	SongCount  int    `json:"songCount"`
	Liked      bool   `json:"liked"`
}

// Playlist is the wire representation of a playlist.
type Playlist struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Comment   string    `json:"comment,omitempty"`
	SongCount int       `json:"songCount"`
	Duration  float32   `json:"duration"`
	Owner     string    `json:"owner"`
	Public    bool      `json:"public"`
	UpdatedAt time.Time `json:"updatedAt"`
}

// PlaylistSong pairs a song with its playlist entry id so it can be removed.
type PlaylistSong struct {
	EntryID string `json:"entryId"`
	Song    Song   `json:"song"`
}

// Genre is the wire representation of a tag genre.
type Genre struct {
	Name       string `json:"name"`
	SongCount  int    `json:"songCount"`
	AlbumCount int    `json:"albumCount"`
}

// UserProfile describes the authenticated user.
type UserProfile struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Username string `json:"username"`
	Email    string `json:"email,omitempty"`
	IsAdmin  bool   `json:"isAdmin"`
}

// Settings exposes app-level configuration to the UI.
type Settings struct {
	AppName     string `json:"appName"`
	Version     string `json:"version"`
	LibraryName string `json:"libraryName"`
	MusicFolder string `json:"musicFolder"`
}

// Section is a home page row of cards.
type Section struct {
	ID     string  `json:"id"`
	Title  string  `json:"title"`
	Albums []Album `json:"albums,omitempty"`
	Songs  []Song  `json:"songs,omitempty"`
	Genres []Genre `json:"genres,omitempty"`
}

// Home aggregates the sections shown on the home page.
type Home struct {
	Sections []Section `json:"sections"`
	Genres   []Genre   `json:"genres"`
}

// SearchResults groups matches by entity type.
type SearchResults struct {
	Songs     []Song     `json:"songs"`
	Albums    []Album    `json:"albums"`
	Artists   []Artist   `json:"artists"`
	Playlists []Playlist `json:"playlists"`
}

type AlbumDetail struct {
	Album
	Songs []Song `json:"songs"`
}

type ArtistDetail struct {
	Artist
	Albums   []Album `json:"albums"`
	TopSongs []Song  `json:"topSongs"`
}

type PlaylistDetail struct {
	Playlist
	Songs []PlaylistSong `json:"songs"`
}

type LyricsResponse struct {
	Synced bool        `json:"synced"`
	Source string      `json:"source,omitempty"`
	Lines  []LyricLine `json:"lines"`
}

type LyricLine struct {
	Start *int64 `json:"start,omitempty"`
	End   *int64 `json:"end,omitempty"`
	Text  string `json:"text"`
}

func toSong(mf *model.MediaFile) Song {
	return Song{
		ID:           mf.ID,
		Title:        mf.Title,
		AlbumID:      mf.AlbumID,
		Album:        mf.Album,
		ArtistID:     mf.ArtistID,
		Artist:       mf.Artist,
		AlbumArtist:  mf.AlbumArtist,
		Duration:     mf.Duration,
		TrackNumber:  mf.TrackNumber,
		DiscNumber:   mf.DiscNumber,
		Year:         mf.Year,
		Genre:        mf.Genre,
		Format:       mf.Suffix,
		BitRate:      mf.BitRate,
		PlayCount:    mf.PlayCount,
		Liked:        mf.Starred,
		LastPlayedAt: mf.PlayDate,
	}
}

func toAlbum(al *model.Album) Album {
	genre := ""
	if len(al.Genres) > 0 {
		genre = al.Genres[0].Name
	}
	year := al.MaxYear
	if year == 0 {
		year = al.MinYear
	}
	return Album{
		ID:        al.ID,
		Name:      al.Name,
		ArtistID:  al.AlbumArtistID,
		Artist:    al.AlbumArtist,
		Year:      year,
		Genre:     genre,
		SongCount: al.SongCount,
		Duration:  al.Duration,
		PlayCount: al.PlayCount,
		Liked:     al.Starred,
		CreatedAt: al.UpdatedAt,
	}
}

func toArtist(ar *model.Artist) Artist {
	return Artist{
		ID:         ar.ID,
		Name:       ar.Name,
		AlbumCount: ar.AlbumCount,
		SongCount:  ar.SongCount,
		Liked:      ar.Starred,
	}
}

func toPlaylist(pls *model.Playlist) Playlist {
	return Playlist{
		ID:        pls.ID,
		Name:      pls.Name,
		Comment:   pls.Comment,
		SongCount: pls.SongCount,
		Duration:  pls.Duration,
		Owner:     pls.OwnerName,
		Public:    pls.Public,
		UpdatedAt: pls.UpdatedAt,
	}
}

func mapSongs(mfs model.MediaFiles) []Song {
	songs := make([]Song, 0, len(mfs))
	for i := range mfs {
		songs = append(songs, toSong(&mfs[i]))
	}
	return songs
}

func mapAlbums(als model.Albums) []Album {
	albums := make([]Album, 0, len(als))
	for i := range als {
		albums = append(albums, toAlbum(&als[i]))
	}
	return albums
}

func mapArtists(ars model.Artists) []Artist {
	artists := make([]Artist, 0, len(ars))
	for i := range ars {
		artists = append(artists, toArtist(&ars[i]))
	}
	return artists
}

func mapPlaylists(pls model.Playlists) []Playlist {
	out := make([]Playlist, 0, len(pls))
	for i := range pls {
		out = append(out, toPlaylist(&pls[i]))
	}
	return out
}
