package ffmpeg

import (
	"testing"

	"github.com/hensi01/play-music/log"
	"github.com/hensi01/play-music/tests"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestFFMpeg(t *testing.T) {
	tests.Init(t, true)
	log.SetLevel(log.LevelFatal)
	RegisterFailHandler(Fail)
	RunSpecs(t, "FFMpeg Suite")
}
