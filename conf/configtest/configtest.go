package configtest

import "github.com/hensi01/play-music/conf"

// TODO Remove this redirection and call SnapshotConfig directly from tests
func SetupConfig() func() {
	return conf.SnapshotConfig()
}
