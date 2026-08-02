package cmd

import (
	"context"
	"errors"
	"fmt"

	"github.com/hensi01/play-music/core/auth"
	"github.com/hensi01/play-music/db"
	"github.com/hensi01/play-music/log"
	"github.com/hensi01/play-music/model"
	"github.com/hensi01/play-music/model/request"
	"github.com/hensi01/play-music/persistence"
)

func getAdminContext(ctx context.Context) (model.DataStore, context.Context) {
	sqlDB := db.Db()
	ds := persistence.New(sqlDB)
	ctx = auth.WithAdminUser(ctx, ds)
	u, _ := request.UserFrom(ctx)
	if !u.IsAdmin {
		log.Fatal(ctx, "There must be at least one admin user to run this command.")
	}
	return ds, ctx
}

func getUser(ctx context.Context, id string, ds model.DataStore) (*model.User, error) {
	user, err := ds.User(ctx).FindByUsername(id)

	if err != nil && !errors.Is(err, model.ErrNotFound) {
		return nil, fmt.Errorf("finding user by name: %w", err)
	}

	if errors.Is(err, model.ErrNotFound) {
		user, err = ds.User(ctx).Get(id)
		if err != nil {
			return nil, fmt.Errorf("finding user by id: %w", err)
		}
	}

	return user, nil
}
