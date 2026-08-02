package core

import (
	"github.com/google/wire"
	"github.com/hensi01/play-music/core/agents"
	"github.com/hensi01/play-music/core/external"
	"github.com/hensi01/play-music/core/ffmpeg"
	"github.com/hensi01/play-music/core/lyrics"
	"github.com/hensi01/play-music/core/matcher"
	"github.com/hensi01/play-music/core/metrics"
	"github.com/hensi01/play-music/core/playback"
	"github.com/hensi01/play-music/core/playlists"
	"github.com/hensi01/play-music/core/scrobbler"
	"github.com/hensi01/play-music/core/stream"
)

var Set = wire.NewSet(
	stream.NewMediaStreamer,
	stream.GetTranscodingCache,
	NewArchiver,
	NewPlayers,
	NewShare,
	playlists.NewPlaylists,
	NewLibrary,
	NewUser,
	NewMaintenance,
	NewImageUploadService,
	wire.Bind(new(playlists.ImageUploadService), new(ImageUploadService)),
	stream.NewTranscodeDecider,
	agents.GetAgents,
	external.NewProvider,
	matcher.New,
	wire.Bind(new(external.Agents), new(*agents.Agents)),
	ffmpeg.New,
	scrobbler.GetPlayTracker,
	playback.GetInstance,
	metrics.GetInstance,
	lyrics.NewLyrics,
)
