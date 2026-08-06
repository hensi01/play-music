package model

import "time"

// Song is the JSON shape returned by the API. The web UI relies on
// id, title, artist, artistId, album, albumId, duration, format and liked.
type Song struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Artist      string    `json:"artist"`
	ArtistID    string    `json:"artistId,omitempty"`
	Album       string    `json:"album"`
	AlbumID     string    `json:"albumId,omitempty"`
	Year        int       `json:"year"`
	Genre       string    `json:"genre,omitempty"`
	Duration    float64   `json:"duration"`
	Format      string    `json:"format"`
	Bitrate     int       `json:"bitrate,omitempty"`
	SampleRate  int       `json:"sampleRate,omitempty"`
	TrackNumber int       `json:"trackNumber,omitempty"`
	DiscNumber  int       `json:"discNumber,omitempty"`
	Path        string    `json:"path,omitempty"`
	Size        int64     `json:"size,omitempty"`
	PlayCount   int64     `json:"playCount"`
	Liked       bool      `json:"liked"`
	HasCover    bool      `json:"-"`
	Lyrics      string    `json:"-"`
	CreatedAt   time.Time `json:"createdAt,omitempty"`
	UpdatedAt   time.Time `json:"updatedAt,omitempty"`
}

type Album struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Artist    string    `json:"artist"`
	ArtistID  string    `json:"artistId,omitempty"`
	Year      int       `json:"year"`
	SongCount int       `json:"songCount"`
	Duration  float64   `json:"duration,omitempty"`
	Liked     bool      `json:"liked"`
	CreatedAt time.Time `json:"createdAt,omitempty"`
	Songs     []Song    `json:"songs,omitempty"`
}

type Artist struct {
	ID         string `json:"id"`
	Name       string `json:"name"`
	AlbumCount int    `json:"albumCount"`
	SongCount  int    `json:"songCount"`
	Liked      bool   `json:"liked"`
}

type PlaylistEntry struct {
	EntryID string `json:"entryId"`
	Song    Song   `json:"song"`
}

type Playlist struct {
	ID        string          `json:"id"`
	Name      string          `json:"name"`
	Comment   string          `json:"comment,omitempty"`
	Owner     string          `json:"owner"`
	UserID    string          `json:"-"`
	SongCount int             `json:"songCount"`
	Duration  float64         `json:"duration,omitempty"`
	Songs     []PlaylistEntry `json:"songs,omitempty"`
}

type LyricsLine struct {
	Text  string `json:"text"`
	Start *int64 `json:"start,omitempty"`
}

type Lyrics struct {
	Synced bool         `json:"synced"`
	Lines  []LyricsLine `json:"lines"`
}

type SearchResults struct {
	Songs      []Song     `json:"songs"`
	Albums     []Album    `json:"albums"`
	Artists    []Artist   `json:"artists"`
	Playlists  []Playlist `json:"playlists"`
	Categories []Category `json:"categories"`
}

type HomeSection struct {
	Title string `json:"title"`
	// Songs is the category/collection content (category -> songs model).
	Songs []Song `json:"songs,omitempty"`
	// Albums kept for API compatibility with legacy album-based sections.
	Albums []Album `json:"albums,omitempty"`
}

type Genre struct {
	Name      string `json:"name"`
	SongCount int    `json:"songCount"`
}

type Home struct {
	Sections []HomeSection `json:"sections"`
	Genres   []Genre       `json:"genres"`
}

type User struct {
	ID         string     `json:"id"`
	Username   string     `json:"username,omitempty"`
	Phone      string     `json:"phone,omitempty"`
	Name       string     `json:"name"`
	IsAdmin    bool       `json:"isAdmin"`
	Categories []Category `json:"categories,omitempty"`
	CreatedAt  time.Time  `json:"createdAt,omitempty"`
}

type Category struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	SongCount int    `json:"songCount,omitempty"`
	Songs     []Song `json:"songs,omitempty"`
}

type Settings struct {
	AppName     string `json:"appName"`
	Version     string `json:"version"`
	LibraryName string `json:"libraryName"`
	MusicFolder string `json:"musicFolder"`
}
