package core

import (
	"context"

	"github.com/deluan/rest"
	"github.com/hensi01/play-music/model"
)

// User provides business logic for user management.
type User interface {
	NewRepository(ctx context.Context) rest.Repository
}

type userService struct {
	ds model.DataStore
}

// NewUser creates a new User service
func NewUser(ds model.DataStore) User {
	return &userService{
		ds: ds,
	}
}

// NewRepository returns a REST repository wrapper for user operations.
func (s *userService) NewRepository(ctx context.Context) rest.Repository {
	return s.ds.User(ctx)
}
