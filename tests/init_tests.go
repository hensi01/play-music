package tests

import (
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"testing"

	"github.com/hensi01/play-music/conf"
	"github.com/hensi01/play-music/log"
)

var once sync.Once

func Init(t *testing.T, skipOnShort bool) {
	if skipOnShort && testing.Short() {
		t.Skip("skipping test in short mode.")
	}
	once.Do(func() {
		_, file, _, _ := runtime.Caller(0)
		appPath, _ := filepath.Abs(filepath.Join(filepath.Dir(file), ".."))
		confPath, _ := filepath.Abs(filepath.Join(appPath, "tests", "playmusic-test.toml"))
		println("Loading test configuration file from " + confPath)
		_ = os.Chdir(appPath)
		conf.LoadFromFile(confPath)

		noLog := os.Getenv("NOLOG")
		if noLog != "" {
			log.SetLevel(log.LevelError)
		}
	})
}
