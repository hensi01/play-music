package metrics

import (
	"context"
	"net/http"

	"github.com/hensi01/play-music/model"
)

// Metrics is the internal instrumentation recorder used by the scanner and
// the API. Prometheus exposure was removed; this interface is kept so internal
// components can record activity without a telemetry backend.
type Metrics interface {
	WriteInitialMetrics(ctx context.Context)
	WriteAfterScanMetrics(ctx context.Context, success bool)
	RecordRequest(ctx context.Context, endpoint, method, client string, status int32, elapsed int64)
	RecordPluginRequest(ctx context.Context, plugin, method string, ok bool, elapsed int64)
	GetHandler() http.Handler
}

func GetPrometheusInstance(_ model.DataStore) Metrics {
	return noopMetrics{}
}

func NewNoopInstance() Metrics {
	return noopMetrics{}
}

type noopMetrics struct {
}

func (n noopMetrics) WriteInitialMetrics(context.Context) {}

func (n noopMetrics) WriteAfterScanMetrics(context.Context, bool) {}

func (n noopMetrics) RecordRequest(context.Context, string, string, string, int32, int64) {}

func (n noopMetrics) RecordPluginRequest(context.Context, string, string, bool, int64) {}

func (n noopMetrics) GetHandler() http.Handler { return nil }
