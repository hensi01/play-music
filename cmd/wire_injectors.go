//go:build wireinject

package cmd

import (
	"context"

	"github.com/google/wire"
	"github.com/hensi01/play-music/core"
	"github.com/hensi01/play-music/core/artwork"
	"github.com/hensi01/play-music/core/metrics"
	"github.com/hensi01/play-music/db"
	"github.com/hensi01/play-music/model"
	"github.com/hensi01/play-music/persistence"
	"github.com/hensi01/play-music/scanner"
	"github.com/hensi01/play-music/server"
	"github.com/hensi01/play-music/server/events"
)

var allProviders = wire.NewSet(
	core.Set,
	artwork.Set,
	server.New,
	persistence.New,
	events.GetBroker,
	scanner.New,
	scanner.GetWatcher,
	metrics.GetPrometheusInstance,
	db.Db,
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
