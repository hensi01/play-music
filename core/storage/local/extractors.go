package local

import (
	"io/fs"
	"sync"

	"github.com/navidrome/navidrome/conf"
	"github.com/navidrome/navidrome/consts"
	"github.com/navidrome/navidrome/log"
	"github.com/navidrome/navidrome/model/metadata"
)

// Extractor is an interface that defines the methods that a tag/metadata extractor must implement
type Extractor interface {
	Parse(files ...string) (map[string]metadata.Info, error)
	Version() string
}

type extractorConstructor func(fs.FS, string) Extractor

var (
	extractors = map[string]extractorConstructor{}
	lock       sync.RWMutex
)

// RegisterExtractor registers a new extractor, so it can be used by the local storage. The one to be used is
// defined with the configuration option Scanner.Extractor.
func RegisterExtractor(id string, f extractorConstructor) {
	lock.Lock()
	defer lock.Unlock()
	extractors[id] = f
}

// NewExtractor returns a new Extractor for the configured Scanner.Extractor, using the
// provided fs.FS as the data source. It falls back to the default extractor when the
// configured one is not registered. This helper allows non-local storage backends
// (e.g. S3/MinIO) to reuse the same tag extraction pipeline.
func NewExtractor(fsys fs.FS, baseDir string) Extractor {
	lock.RLock()
	newExtractor, ok := extractors[conf.Server.Scanner.Extractor]
	lock.RUnlock()
	if !ok || newExtractor == nil {
		if conf.Server.Scanner.Extractor != consts.DefaultScannerExtractor {
			log.Warn("Extractor not found, using default", "extractor", conf.Server.Scanner.Extractor, "default", consts.DefaultScannerExtractor)
		}
		lock.RLock()
		newExtractor = extractors[consts.DefaultScannerExtractor]
		lock.RUnlock()
		if newExtractor == nil {
			log.Fatal("Default extractor not registered", "extractor", consts.DefaultScannerExtractor)
		}
	}
	return newExtractor(fsys, baseDir)
}
