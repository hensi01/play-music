package model_test

import (
	"testing"

	_ "github.com/mattn/go-sqlite3"
	"github.com/hensi01/play-music/log"
	"github.com/hensi01/play-music/tests"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestModel(t *testing.T) {
	tests.Init(t, true)
	log.SetLevel(log.LevelFatal)
	RegisterFailHandler(Fail)
	RunSpecs(t, "Model Suite")
}
