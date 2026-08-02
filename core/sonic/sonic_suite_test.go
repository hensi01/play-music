package sonic_test

import (
	"testing"

	"github.com/hensi01/play-music/log"
	"github.com/hensi01/play-music/tests"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestSonic(t *testing.T) {
	tests.Init(t, false)
	log.SetLevel(log.LevelFatal)
	RegisterFailHandler(Fail)
	RunSpecs(t, "Sonic Suite")
}
