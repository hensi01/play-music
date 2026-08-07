package server

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"play-music/internal/artwork"
	"play-music/internal/auth"
	"play-music/internal/config"
	"play-music/internal/scanner"
	"play-music/internal/storage"
	"play-music/internal/store"
	"play-music/internal/stream"
)

type Server struct {
	cfg     *config.Config
	store   *store.Store
	auth    *auth.Auth
	stream  *stream.Service
	storage *storage.Storage
	artwork *artwork.Service
	scanner *scanner.Scanner
	log     *slog.Logger
}

type Dependencies struct {
	Config  *config.Config
	Store   *store.Store
	Auth    *auth.Auth
	Stream  *stream.Service
	Storage *storage.Storage
	Artwork *artwork.Service
	Scanner *scanner.Scanner
	Log     *slog.Logger
}

func New(deps Dependencies) *Server {
	return &Server{
		cfg:     deps.Config,
		store:   deps.Store,
		auth:    deps.Auth,
		stream:  deps.Stream,
		storage: deps.Storage,
		artwork: deps.Artwork,
		scanner: deps.Scanner,
		log:     deps.Log,
	}
}

var nativeFormats = map[string]bool{
	"mp3": true, "m4a": true, "aac": true, "ogg": true, "opus": true,
	"wav": true, "flac": true,
}

// Handler builds the HTTP handler with all routes and middleware.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()

	// Auth (public).
	mux.HandleFunc("POST /auth/login", s.handleLogin)

	// Store (public): phone registration + category releases after checkout.
	mux.HandleFunc("GET /api/store/categories", s.handleStoreCategories)
	mux.HandleFunc("POST /api/store/register", s.handleStoreRegister)
	mux.Handle("POST /api/store/purchase", s.requireAuth(http.HandlerFunc(s.handleStorePurchase)))

	// API (JWT required).
	mux.Handle("GET /api/me", s.requireAuth(http.HandlerFunc(s.handleMe)))
	mux.Handle("GET /api/settings", s.requireAuth(http.HandlerFunc(s.handleSettings)))
	mux.Handle("GET /api/home", s.requireAuth(http.HandlerFunc(s.handleHome)))
	mux.Handle("GET /api/search", s.requireAuth(http.HandlerFunc(s.handleSearch)))
	mux.Handle("GET /api/categories", s.requireAuth(http.HandlerFunc(s.handleCategories)))
	mux.Handle("GET /api/categories/{id}", s.requireAuth(http.HandlerFunc(s.handleCategory)))

	mux.Handle("GET /api/albums", s.requireAuth(http.HandlerFunc(s.handleAlbums)))
	mux.Handle("GET /api/albums/{id}", s.requireAuth(http.HandlerFunc(s.handleAlbum)))
	mux.Handle("GET /api/artists", s.requireAuth(http.HandlerFunc(s.handleArtists)))
	mux.Handle("GET /api/artists/{id}", s.requireAuth(http.HandlerFunc(s.handleArtist)))
	mux.Handle("GET /api/songs", s.requireAuth(http.HandlerFunc(s.handleSongs)))
	mux.Handle("GET /api/songs/{id}", s.requireAuth(http.HandlerFunc(s.handleSong)))

	mux.Handle("GET /api/playlists", s.requireAuth(http.HandlerFunc(s.handlePlaylists)))
	mux.Handle("POST /api/playlists", s.requireAuth(http.HandlerFunc(s.handleCreatePlaylist)))
	mux.Handle("GET /api/playlists/{id}", s.requireAuth(http.HandlerFunc(s.handlePlaylist)))
	mux.Handle("PUT /api/playlists/{id}", s.requireAuth(http.HandlerFunc(s.handleUpdatePlaylist)))
	mux.Handle("DELETE /api/playlists/{id}", s.requireAuth(http.HandlerFunc(s.handleDeletePlaylist)))
	mux.Handle("POST /api/playlists/{id}/tracks", s.requireAuth(http.HandlerFunc(s.handleAddPlaylistTracks)))
	mux.Handle("DELETE /api/playlists/{id}/tracks/{entryId}", s.requireAuth(http.HandlerFunc(s.handleRemovePlaylistTrack)))
	mux.Handle("PUT /api/playlists/{id}/tracks", s.requireAuth(http.HandlerFunc(s.handleReorderPlaylistTracks)))

	mux.Handle("GET /api/me/liked", s.requireAuth(http.HandlerFunc(s.handleLiked)))
	mux.Handle("PUT /api/me/liked/{id}", s.requireAuth(http.HandlerFunc(s.handleLike)))
	mux.Handle("DELETE /api/me/liked/{id}", s.requireAuth(http.HandlerFunc(s.handleUnlike)))
	mux.Handle("GET /api/me/history", s.requireAuth(http.HandlerFunc(s.handleHistory)))
	mux.Handle("POST /api/me/history/{id}", s.requireAuth(http.HandlerFunc(s.handleRegisterPlay)))

	mux.Handle("GET /api/queue", s.requireAuth(http.HandlerFunc(s.handleGetQueue)))
	mux.Handle("PUT /api/queue", s.requireAuth(http.HandlerFunc(s.handleSaveQueue)))

	// Media (JWT via header or ?jwt= query) — access-guarded.
	mux.Handle("GET /api/artwork/{id}", s.requireAuth(http.HandlerFunc(s.handleArtwork)))
	mux.Handle("GET /api/stream/{id}", s.requireAuth(http.HandlerFunc(s.handleStream)))

	// Admin (JWT + is_admin).
	mux.Handle("GET /api/admin/users", s.requireAdmin(http.HandlerFunc(s.handleAdminListUsers)))
	mux.Handle("POST /api/admin/users", s.requireAdmin(http.HandlerFunc(s.handleAdminCreateUser)))
	mux.Handle("PUT /api/admin/users/{id}", s.requireAdmin(http.HandlerFunc(s.handleAdminUpdateUser)))
	mux.Handle("DELETE /api/admin/users/{id}", s.requireAdmin(http.HandlerFunc(s.handleAdminDeleteUser)))
	mux.Handle("GET /api/admin/categories", s.requireAdmin(http.HandlerFunc(s.handleAdminListCategories)))
	mux.Handle("POST /api/admin/categories", s.requireAdmin(http.HandlerFunc(s.handleAdminCreateCategory)))
	mux.Handle("GET /api/admin/categories/{id}", s.requireAdmin(http.HandlerFunc(s.handleAdminCategoryDetail)))
	mux.Handle("PUT /api/admin/categories/{id}", s.requireAdmin(http.HandlerFunc(s.handleAdminUpdateCategory)))
	mux.Handle("DELETE /api/admin/categories/{id}", s.requireAdmin(http.HandlerFunc(s.handleAdminDeleteCategory)))
	mux.Handle("GET /api/admin/albums", s.requireAdmin(http.HandlerFunc(s.handleAdminAlbums)))
	mux.Handle("GET /api/admin/artists", s.requireAdmin(http.HandlerFunc(s.handleAdminArtists)))
	mux.Handle("GET /api/admin/songs", s.requireAdmin(http.HandlerFunc(s.handleAdminSongs)))
	mux.Handle("POST /api/admin/songs", s.requireAdmin(http.HandlerFunc(s.handleAdminUploadSong)))
	mux.Handle("POST /api/admin/albums/{id}/photo", s.requireAdmin(http.HandlerFunc(s.handleAdminUploadPhoto)))
	mux.Handle("DELETE /api/admin/albums/{id}/photo", s.requireAdmin(http.HandlerFunc(s.handleAdminDeletePhoto)))
	mux.Handle("POST /api/admin/songs/{id}/photo", s.requireAdmin(http.HandlerFunc(s.handleAdminUploadSongPhoto)))
	mux.Handle("DELETE /api/admin/songs/{id}/photo", s.requireAdmin(http.HandlerFunc(s.handleAdminDeleteSongPhoto)))
	mux.Handle("POST /api/admin/categories/{id}/photo", s.requireAdmin(http.HandlerFunc(s.handleAdminUploadCategoryPhoto)))
	mux.Handle("DELETE /api/admin/categories/{id}/photo", s.requireAdmin(http.HandlerFunc(s.handleAdminDeleteCategoryPhoto)))
	mux.Handle("POST /api/scan", s.requireAdmin(http.HandlerFunc(s.handleScan)))

	// Static UI.
	mux.Handle("/", s.handleStatic())

	return s.middleware(mux)
}

// requireAuth validates the JWT (header or ?jwt=), stores the user in the
// context and refreshes the token when past half its lifetime.
func (s *Server) requireAuth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := s.auth.TokenFromRequest(r)
		if token == "" {
			writeError(w, http.StatusUnauthorized, "Não autenticado")
			return
		}
		claims, err := s.auth.Parse(token)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "Não autenticado")
			return
		}
		if s.auth.NeedsRefresh(claims) {
			if fresh, err := s.auth.Sign(r.Context(), modelUserFromClaims(claims)); err == nil {
				w.Header().Set(auth.HeaderName, fresh)
			}
		}
		ctx := context.WithValue(r.Context(), ctxUserID, claims.UserID)
		ctx = context.WithValue(ctx, ctxIsAdmin, claims.IsAdmin)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// requireAdmin requires a valid JWT with the admin flag.
func (s *Server) requireAdmin(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		token := s.auth.TokenFromRequest(r)
		if token == "" {
			writeError(w, http.StatusUnauthorized, "Não autenticado")
			return
		}
		claims, err := s.auth.Parse(token)
		if err != nil {
			writeError(w, http.StatusUnauthorized, "Não autenticado")
			return
		}
		if !claims.IsAdmin {
			writeError(w, http.StatusForbidden, "Sem permissão")
			return
		}
		if s.auth.NeedsRefresh(claims) {
			if fresh, err := s.auth.Sign(r.Context(), modelUserFromClaims(claims)); err == nil {
				w.Header().Set(auth.HeaderName, fresh)
			}
		}
		ctx := context.WithValue(r.Context(), ctxUserID, claims.UserID)
		ctx = context.WithValue(ctx, ctxIsAdmin, true)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

// middleware applies CORS, logging and panic recovery.
func (s *Server) middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, X-ND-Authorization, Authorization")
		w.Header().Set("Access-Control-Expose-Headers", auth.HeaderName)
		if r.Method == http.MethodOptions {
			w.WriteHeader(http.StatusNoContent)
			return
		}

		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		defer func() {
			if p := recover(); p != nil {
				s.log.Error("panic", "err", p, "path", r.URL.Path)
				if rec.status == http.StatusOK {
					writeError(rec, http.StatusInternalServerError, "Erro interno")
				}
			}
			s.log.Info("request",
				"method", r.Method,
				"path", r.URL.Path,
				"status", rec.status,
				"duration", time.Since(start).Round(time.Millisecond).String(),
			)
		}()

		next.ServeHTTP(rec, r)
	})
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(status int) {
	r.status = status
	r.ResponseWriter.WriteHeader(status)
}

func (r *statusRecorder) Write(b []byte) (int, error) {
	if r.status == 0 {
		r.status = http.StatusOK
	}
	return r.ResponseWriter.Write(b)
}
