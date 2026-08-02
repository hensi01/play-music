package cmd

import (
	"context"

	"github.com/hensi01/play-music/core"
	"github.com/hensi01/play-music/core/agents"
	"github.com/hensi01/play-music/core/artwork"
	"github.com/hensi01/play-music/core/external"
	"github.com/hensi01/play-music/core/ffmpeg"
	"github.com/hensi01/play-music/core/lyrics"
	"github.com/hensi01/play-music/core/matcher"
	"github.com/hensi01/play-music/core/metrics"
	"github.com/hensi01/play-music/core/playlists"
	"github.com/hensi01/play-music/core/scrobbler"
	"github.com/hensi01/play-music/core/stream"
	"github.com/hensi01/play-music/db"
	"github.com/hensi01/play-music/persistence"
	"github.com/hensi01/play-music/plugins"
	"github.com/hensi01/play-music/server/api"
	"github.com/hensi01/play-music/server/events"
)

// CreateAPIRouter builds the Play Music REST API router with its dependency
// graph. Kept as a hand-written constructor (mirroring the generated wire
// output) so the API does not depend on the wire codegen toolchain.
func CreateAPIRouter(ctx context.Context) *api.Router {
	sqlDB := db.Db()
	dataStore := persistence.New(sqlDB)
	fileCache := artwork.GetImageCache()
	fFmpeg := ffmpeg.New()
	broker := events.GetBroker()
	metricsMetrics := metrics.GetPrometheusInstance(dataStore)
	manager := plugins.GetManager(dataStore, broker, metricsMetrics)
	agentsAgents := agents.GetAgents(dataStore, manager)
	matcherMatcher := matcher.New(dataStore)
	provider := external.NewProvider(dataStore, agentsAgents, matcherMatcher)
	artworkArtwork := artwork.NewArtwork(dataStore, fileCache, fFmpeg, provider)
	transcodingCache := stream.GetTranscodingCache()
	mediaStreamer := stream.NewMediaStreamer(dataStore, fFmpeg, transcodingCache)
	imageUploadService := core.NewImageUploadService()
	playlistsPlaylists := playlists.NewPlaylists(dataStore, imageUploadService)
	playTracker := scrobbler.GetPlayTracker(dataStore, broker, manager)
	transcodeDecider := stream.NewTranscodeDecider(dataStore, fFmpeg)
	lyricsLyrics := lyrics.NewLyrics(dataStore, manager)
	return api.New(dataStore, artworkArtwork, mediaStreamer, transcodeDecider, playlistsPlaylists, playTracker, lyricsLyrics)
}
