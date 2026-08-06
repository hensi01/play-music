package auth

import (
	"context"
	"crypto/subtle"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"

	"play-music/internal/config"
	"play-music/internal/model"
	"play-music/internal/store"
)

const (
	// Header carries the bearer token; the response header may carry a
	// refreshed token (the web UI relies on X-ND-Authorization).
	HeaderName = "X-ND-Authorization"

	tokenTTL = 24 * time.Hour
)

type Claims struct {
	Username string `json:"username"`
	Name     string `json:"name"`
	IsAdmin  bool   `json:"isAdmin"`
	jwt.RegisteredClaims
}

type Auth struct {
	cfg        *config.Config
	store      *store.Store
	secret     []byte
	adminHash  []byte
}

func New(ctx context.Context, cfg *config.Config, st *store.Store) (*Auth, error) {
	secret, err := st.GetOrCreateSecret(ctx)
	if err != nil {
		return nil, err
	}
	a := &Auth{cfg: cfg, store: st, secret: secret}
	if cfg.AdminPassword != "" {
		a.adminHash = []byte(cfg.AdminPassword)
	}
	return a, nil
}

// AdminUser returns the single admin account (credentials from the env).
func (a *Auth) AdminUser() model.User {
	return model.User{
		ID:       "1",
		Username: a.cfg.AdminUsername,
		Name:     a.cfg.AdminUsername,
		IsAdmin:  true,
	}
}

// Login validates the credentials (single source: the environment) and returns
// the user plus a signed token.
func (a *Auth) Login(ctx context.Context, username, password string) (model.User, string, error) {
	if a.cfg.AdminUsername == "" || a.cfg.AdminPassword == "" {
		return model.User{}, "", errors.New("credenciais de administrador não configuradas")
	}
	userOK := subtle.ConstantTimeCompare([]byte(username), []byte(a.cfg.AdminUsername)) == 1
	passOK := subtle.ConstantTimeCompare([]byte(password), a.adminHash) == 1
	if !userOK || !passOK {
		return model.User{}, "", errors.New("usuário ou senha inválidos")
	}
	token, err := a.Sign(ctx, username)
	if err != nil {
		return model.User{}, "", err
	}
	return a.AdminUser(), token, nil
}

func (a *Auth) Sign(ctx context.Context, username string) (string, error) {
	now := time.Now()
	claims := Claims{
		Username: username,
		Name:     username,
		IsAdmin:  true,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   username,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(tokenTTL)),
		},
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return tok.SignedString(a.secret)
}

// Parse validates a token string and returns the claims.
func (a *Auth) Parse(tokenString string) (*Claims, error) {
	claims := &Claims{}
	_, err := jwt.ParseWithClaims(tokenString, claims, func(t *jwt.Token) (any, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("método de assinatura inválido")
		}
		return a.secret, nil
	}, jwt.WithValidMethods([]string{"HS256"}))
	if err != nil {
		return nil, err
	}
	return claims, nil
}

// NeedsRefresh reports whether the token is past half its lifetime.
func (a *Auth) NeedsRefresh(c *Claims) bool {
	if c.ExpiresAt == nil {
		return false
	}
	remain := time.Until(c.ExpiresAt.Time)
	return remain < tokenTTL/2
}

// TokenFromRequest extracts the token from the X-ND-Authorization /
// Authorization headers or from the ?jwt= query parameter (used by <img> and
// <audio> tags, which cannot send headers).
func (a *Auth) TokenFromRequest(r *http.Request) string {
	if v := r.Header.Get(HeaderName); v != "" {
		return strings.TrimPrefix(v, "Bearer ")
	}
	if v := r.Header.Get("Authorization"); v != "" {
		return strings.TrimPrefix(v, "Bearer ")
	}
	return r.URL.Query().Get("jwt")
}
