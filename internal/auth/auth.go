package auth

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"

	"play-music/internal/config"
	"play-music/internal/model"
	"play-music/internal/store"
)

const (
	// HeaderName carries the bearer token; the response header may carry a
	// refreshed token (the web UI relies on X-ND-Authorization).
	HeaderName = "X-ND-Authorization"

	tokenTTL = 24 * time.Hour
)

type Claims struct {
	UserID   string `json:"uid"`
	Username string `json:"username,omitempty"`
	Phone    string `json:"phone,omitempty"`
	Name     string `json:"name"`
	IsAdmin  bool   `json:"isAdmin"`
	jwt.RegisteredClaims
}

type Auth struct {
	cfg    *config.Config
	store  *store.Store
	secret []byte
}

// New loads the JWT secret and bootstraps the admin account from the
// environment on first boot (single source: ND_ADMINUSERNAME/ND_ADMINPASSWORD).
func New(ctx context.Context, cfg *config.Config, st *store.Store) (*Auth, error) {
	secret, err := st.GetOrCreateSecret(ctx)
	if err != nil {
		return nil, err
	}
	a := &Auth{cfg: cfg, store: st, secret: secret}
	if err := a.bootstrapAdmin(ctx); err != nil {
		return nil, err
	}
	return a, nil
}

func (a *Auth) bootstrapAdmin(ctx context.Context) error {
	has, err := a.store.HasAdmin(ctx)
	if err != nil {
		return err
	}
	if has {
		return nil
	}
	if a.cfg.AdminUsername == "" || a.cfg.AdminPassword == "" {
		return errors.New("nenhum administrador existe e ND_ADMINUSERNAME/ND_ADMINPASSWORD não estão configuradas")
	}
	hash, err := bcrypt.GenerateFromPassword([]byte(a.cfg.AdminPassword), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	admin := &model.User{
		ID:       store.NewID(),
		Username: a.cfg.AdminUsername,
		Name:     a.cfg.AdminUsername,
		IsAdmin:  true,
	}
	if err := a.store.CreateUser(ctx, admin, string(hash), nil); err != nil {
		return err
	}
	// Migrate legacy rows (playlists, likes, history, queue) to the admin.
	return a.store.BackfillLegacy(ctx, admin.ID)
}

// LoginUsername validates admin credentials (username from env-created account).
func (a *Auth) LoginUsername(ctx context.Context, username, password string) (model.User, string, error) {
	u, hash, err := a.store.GetUserByUsername(ctx, username)
	if errors.Is(err, store.ErrNotFound) {
		return model.User{}, "", errors.New("usuário ou senha inválidos")
	}
	if err != nil {
		return model.User{}, "", err
	}
	if !u.IsAdmin {
		return model.User{}, "", errors.New("usuário ou senha inválidos")
	}
	if bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) != nil {
		return model.User{}, "", errors.New("usuário ou senha inválidos")
	}
	token, err := a.Sign(ctx, *u)
	if err != nil {
		return model.User{}, "", err
	}
	return *u, token, nil
}

// LoginPhone authenticates a client by phone number only (no password —
// access is granted exclusively by the admin creating the account).
func (a *Auth) LoginPhone(ctx context.Context, phone string) (model.User, string, error) {
	u, _, err := a.store.GetUserByPhone(ctx, phone)
	if errors.Is(err, store.ErrNotFound) {
		return model.User{}, "", errors.New("telefone não cadastrado")
	}
	if err != nil {
		return model.User{}, "", err
	}
	if u.IsAdmin {
		return model.User{}, "", errors.New("telefone não cadastrado")
	}
	token, err := a.Sign(ctx, *u)
	if err != nil {
		return model.User{}, "", err
	}
	return *u, token, nil
}

func (a *Auth) Sign(ctx context.Context, u model.User) (string, error) {
	now := time.Now()
	claims := Claims{
		UserID:  u.ID,
		Username: u.Username,
		Phone:   u.Phone,
		Name:    u.Name,
		IsAdmin: u.IsAdmin,
		RegisteredClaims: jwt.RegisteredClaims{
			Subject:   u.ID,
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
