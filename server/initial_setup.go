package server

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/hensi01/play-music/conf"
	"github.com/hensi01/play-music/consts"
	"github.com/hensi01/play-music/core/ffmpeg"
	"github.com/hensi01/play-music/log"
	"github.com/hensi01/play-music/model"
)

func initialSetup(ds model.DataStore) {
	ctx := context.TODO()
	_ = ds.WithTx(func(tx model.DataStore) error {
		if err := syncManagedAdmin(tx); err != nil {
			return err
		}
		if err := tx.Library(ctx).StoreMusicFolder(); err != nil {
			return err
		}

		properties := tx.Property(ctx)
		_, err := properties.Get(consts.InitialSetupFlagKey)
		if err == nil {
			return nil
		}
		log.Info("Running initial setup")
		err = properties.Put(consts.InitialSetupFlagKey, time.Now().String())
		return err
	}, "initial setup")
}

// syncManagedAdmin creates or updates the administrator declared in the environment.
// Keeping this on every startup makes the environment the source of truth for both credentials.
func syncManagedAdmin(ds model.DataStore) error {
	username := conf.Server.AdminUsername
	password := conf.Server.AdminPassword
	if username == "" && password == "" {
		return nil
	}
	if username == "" || password == "" {
		return errors.New("ND_ADMINUSERNAME and ND_ADMINPASSWORD must be configured together")
	}

	users := ds.User(context.TODO())
	admin, err := users.FindFirstAdmin()
	if errors.Is(err, model.ErrNotFound) {
		log.Info("Creating environment-managed admin user", "user", username)
		return createAdminUser(context.TODO(), ds, username, password)
	}
	if err != nil {
		return fmt.Errorf("finding managed admin user: %w", err)
	}

	admin.UserName = username
	admin.Name = username
	admin.NewPassword = password
	admin.IsAdmin = true
	if err := users.Put(admin); err != nil {
		return fmt.Errorf("updating managed admin user: %w", err)
	}
	log.Info("Synchronized environment-managed admin user", "user", username)
	return nil
}

func checkFFmpegInstallation() {
	f := ffmpeg.New()
	_, err := f.CmdPath()
	if err != nil {
		log.Warn("Unable to find ffmpeg. Transcoding will fail if used", err)
		if conf.Server.Scanner.Extractor == "ffmpeg" {
			log.Warn("ffmpeg cannot be used for metadata extraction. Falling back to taglib")
			conf.Server.Scanner.Extractor = "taglib"
		}
		return
	}
	if !f.IsProbeAvailable() {
		log.Warn("Unable to find ffprobe. Transcoding decisions will be limited")
	}
}

func checkExternalCredentials() {
	if conf.Server.EnableExternalServices {
		if !conf.Server.LastFM.Enabled {
			log.Info("Last.fm integration is DISABLED")
		} else {
			log.Debug("Last.fm integration is ENABLED")
		}

		if !conf.Server.ListenBrainz.Enabled {
			log.Info("ListenBrainz integration is DISABLED")
		} else {
			log.Debug("ListenBrainz integration is ENABLED", "ListenBrainz.BaseURL", conf.Server.ListenBrainz.BaseURL)
		}
	}
}
