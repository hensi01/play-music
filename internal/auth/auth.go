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

func (a *Auth) LoginUsername(ctx context.Context, username, password string) (model.User, string, error) {
	u, hash, err := a.store.GetUserByUsernameOrEmail(ctx, username)
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
	token, err := a.Sign(*u)
	if err != nil {
		return model.User{}, "", err
	}
	return *u, token, nil
}

func (a *Auth) LoginPhone(ctx context.Context, phone string) (model.User, string, error) {
	u, err := a.store.GetUserByPhone(ctx, phone)
	if errors.Is(err, store.ErrNotFound) {
		return model.User{}, "", errors.New("telefone não cadastrado")
	}
	if err != nil {
		return model.User{}, "", err
	}
	if u.IsAdmin {
		return model.User{}, "", errors.New("telefone não cadastrado")
	}
	token, err := a.Sign(*u)
	if err != nil {
		return model.User{}, "", err
	}
	return *u, token, nil
}

func (a *Auth) Sign(u model.User) (string, error) {
	now := time.Now()
	claims := Claims{
		UserID:   u.ID,
		Username: u.Username,
		Phone:    u.Phone,
		Name:     u.Name,
		IsAdmin:  u.IsAdmin,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(tokenTTL)),
		},
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return tok.SignedString(a.secret)
}

func (a *Auth) Parse(tokenString string) (*Claims, error) {
	claims := &Claims{}
	// WithValidMethods is what rejects a foreign signing method; the keyfunc
	// only supplies the secret.
	_, err := jwt.ParseWithClaims(tokenString, claims, func(*jwt.Token) (any, error) {
		return a.secret, nil
	}, jwt.WithValidMethods([]string{"HS256"}))
	if err != nil {
		return nil, err
	}
	return claims, nil
}

func (a *Auth) NeedsRefresh(c *Claims) bool {
	if c.ExpiresAt == nil {
		return false
	}
	remain := time.Until(c.ExpiresAt.Time)
	return remain < tokenTTL/2
}

// TokenFromRequest also accepts ?jwt=, because <img> and <audio> cannot send
// headers.
func (a *Auth) TokenFromRequest(r *http.Request) string {
	if v := r.Header.Get(HeaderName); v != "" {
		return strings.TrimPrefix(v, "Bearer ")
	}
	if v := r.Header.Get("Authorization"); v != "" {
		return strings.TrimPrefix(v, "Bearer ")
	}
	return r.URL.Query().Get("jwt")
}
