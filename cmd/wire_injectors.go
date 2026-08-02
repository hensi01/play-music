//go:build wireinject

package cmd

import (
	"context"

	"github.com/google/wire"
	"github.com/hensi01/play-music/adapters/lastfm"
	"github.com/hensi01/play-music/adapters/listenbrainz"
	"github.com/hensi01/play-music/core"
	"github.com/hensi01/play-music/core/agents"
	"github.com/hensi01/play-music/core/artwork"
	"github.com/hensi01/play-music/core/lyrics"
	"github.com/hensi01/play-music/core/metrics"
	"github.com/hensi01/play-music/core/playback"
	"github.com/hensi01/play-music/core/scrobbler"
	"github.com/hensi01/play-music/core/sonic"
	"github.com/hensi01/play-music/db"
	"github.com/hensi01/play-music/model"
	"github.com/hensi01/play-music/persistence"
	"github.com/hensi01/play-music/plugins"
	"github.com/hensi01/play-music/scanner"
	"github.com/hensi01/play-music/server"
	"github.com/hensi01/play-music/server/events"
	"github.com/hensi01/play-music/server/jellyfin"
	"github.com/hensi01/play-music/server/nativeapi"
	"github.com/hensi01/play-music/server/public"
	"github.com/hensi01/play-music/server/subsonic"
)

var allProviders = wire.NewSet(
	core.Set,
	artwork.Set,
	server.New,
	subsonic.New,
	jellyfin.New,
	nativeapi.New,
	public.New,
	persistence.New,
	lastfm.NewRouter,
	listenbrainz.NewRouter,
	events.GetBroker,
	scanner.New,
	scanner.GetWatcher,
	metrics.GetPrometheusInstance,
	db.Db,
	plugins.GetManager,
	sonic.New,
	wire.Bind(new(agents.PluginLoader), new(*plugins.Manager)),
	wire.Bind(new(scrobbler.PluginLoader), new(*plugins.Manager)),
	wire.Bind(new(lyrics.PluginLoader), new(*plugins.Manager)),
	wire.Bind(new(sonic.PluginLoader), new(*plugins.Manager)),
	wire.Bind(new(sonic.Engine), new(*sonic.Sonic)),
	wire.Bind(new(nativeapi.PluginManager), new(*plugins.Manager)),
	wire.Bind(new(core.PluginUnloader), new(*plugins.Manager)),
	wire.Bind(new(plugins.PluginMetricsRecorder), new(metrics.Metrics)),
	wire.Bind(new(core.Watcher), new(scanner.Watcher)),
)

func CreateDataStore() model.DataStore {
	panic(wire.Build(
		allProviders,
	))
}

func CreateServer() *server.Server {
	panic(wire.Build(
		allProviders,
	))
}

func CreateNativeAPIRouter(ctx context.Context) *nativeapi.Router {
	panic(wire.Build(
		allProviders,
	))
}

func CreateSubsonicAPIRouter(ctx context.Context) *subsonic.Router {
	panic(wire.Build(
		allProviders,
	))
}

func CreateJellyfinAPIRouter(ctx context.Context) *jellyfin.Router {
	panic(wire.Build(
		allProviders,
	))
}

func CreatePublicRouter() *public.Router {
	panic(wire.Build(
		allProviders,
	))
}

func CreateLastFMRouter() *lastfm.Router {
	panic(wire.Build(
		allProviders,
	))
}

func CreateListenBrainzRouter() *listenbrainz.Router {
	panic(wire.Build(
		allProviders,
	))
}

func CreateInsights() metrics.Insights {
	panic(wire.Build(
		allProviders,
	))
}

func CreatePrometheus() metrics.Metrics {
	panic(wire.Build(
		allProviders,
	))
}

func CreateScanner(ctx context.Context) model.Scanner {
	panic(wire.Build(
		allProviders,
	))
}

func CreateScanWatcher(ctx context.Context) scanner.Watcher {
	panic(wire.Build(
		allProviders,
	))
}

func GetPlaybackServer() playback.PlaybackServer {
	panic(wire.Build(
		allProviders,
	))
}

func getPluginManager() *plugins.Manager {
	panic(wire.Build(
		allProviders,
	))
}

func GetPluginManager(ctx context.Context) *plugins.Manager {
	manager := getPluginManager()
	manager.SetSubsonicRouter(CreateSubsonicAPIRouter(ctx))
	return manager
}
