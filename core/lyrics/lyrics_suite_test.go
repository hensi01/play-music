package lyrics_test

import (
	"io/fs"
	"testing"

	"github.com/hensi01/play-music/consts"
	"github.com/hensi01/play-music/core/storage/local"
	"github.com/hensi01/play-music/log"
	"github.com/hensi01/play-music/model/metadata"
	"github.com/hensi01/play-music/tests"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestLyrics(t *testing.T) {
	tests.Init(t, false)
	log.SetLevel(log.LevelFatal)
	RegisterFailHandler(Fail)
	RunSpecs(t, "Lyrics Suite")
}

// core/storage/local calls log.Fatal if the default scanner extractor is unregistered
// when constructing any localStorage. Register a no-op so storage.For("file://...") works
// in tests without importing the real extractor.
var _ = BeforeSuite(func() {
	local.RegisterExtractor(consts.DefaultScannerExtractor, func(fs.FS, string) local.Extractor {
		return &noopExtractor{}
	})
})

type noopExtractor struct{}

func (e *noopExtractor) Parse(_ ...string) (map[string]metadata.Info, error) { return nil, nil }
func (e *noopExtractor) Version() string                                     { return "noop" }
