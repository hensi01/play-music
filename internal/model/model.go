package model

import "time"

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
	SongCount int             `json:"songCount"`
	Duration  float64         `json:"duration,omitempty"`
	Songs     []PlaylistEntry `json:"songs,omitempty"`
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
	Songs []Song `json:"songs,omitempty"`
	// Albums is never filled by handleHome; it stays in the payload because the
	// published schema declares it and older clients read it.
	Albums []Album `json:"albums,omitempty"`
}

type Genre struct {
	Name      string `json:"name"`
	SongCount int    `json:"songCount"`
}

type Home struct {
	Sections []HomeSection `json:"sections"`
	// Genres is always empty: handleHome has no genre query. The field stays
	// because the published schema declares it.
	Genres []Genre `json:"genres"`
}

type User struct {
	ID         string     `json:"id"`
	Username   string     `json:"username,omitempty"`
	Email      string     `json:"email,omitempty"`
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
	// AudioExtensions is the accepted upload formats, so a client never has to
	// keep its own copy of the list.
	AudioExtensions []string `json:"audioExtensions"`
}

// APIKey is an integration key as the admin screen sees it. The raw key and its
// hash are never part of the listing.
type APIKey struct {
	ID         string     `json:"id"`
	Name       string     `json:"name"`
	CreatedAt  time.Time  `json:"createdAt"`
	LastUsedAt *time.Time `json:"lastUsedAt"`
}

type Download struct {
	ID           string    `json:"id"`
	Title        string    `json:"title"`
	Artist       string    `json:"artist"`
	DownloadedAt time.Time `json:"downloadedAt"`
}

// PushSubscription is an inactive device waiting for remote revocation.
type PushSubscription struct {
	Endpoint         string
	ManagementToken  string
	SubscriptionData []byte
}

// PushCampaign is a scheduled notification. Status is one of
// store.CampaignPending, CampaignSending, CampaignCompleted, CampaignPartial,
// CampaignUnknown or CampaignCancelled.
type PushCampaign struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Body        string    `json:"body"`
	URL         string    `json:"url"`
	ScheduledAt time.Time `json:"scheduledAt"`
	Status      string    `json:"status"`
	Selected    int       `json:"selected"`
	Queued      int       `json:"queued"`
	Detail      string    `json:"detail"`
	RepeatHours int       `json:"repeatHours"`
}

// Notification is one published inbox entry. It is written when a campaign is
// claimed, so the inbox holds only notifications that were actually sent.
type Notification struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Body        string    `json:"body"`
	URL         string    `json:"url"`
	PublishedAt time.Time `json:"publishedAt"`
}
