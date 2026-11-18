package server

import (
	"context"
	"log/slog"
	"net/http"
	"strings"
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

func New(cfg *config.Config, st *store.Store, a *auth.Auth, strm *stream.Service, strg *storage.Storage, art *artwork.Service, sc *scanner.Scanner, log *slog.Logger) *Server {
	return &Server{
		cfg:     cfg,
		store:   st,
		auth:    a,
		stream:  strm,
		storage: strg,
		artwork: art,
		scanner: sc,
		log:     log,
	}
}

// route is one HTTP route: the "METHOD /path" pattern the mux registers and its
// handler.
type route struct {
	Pattern string
	Handler http.HandlerFunc
}

func (s *Server) routes() []route {
	requireAuth := s.requireAuth
	requireAdmin := s.requireAdmin
	return []route{
		// Auth (public).
		{"POST /auth/login", s.handleLogin},
		{"POST /auth/admin/login", s.handleAdminLogin},
		{"POST /api/register", s.handleRegister},

		// Push.
		{"POST /api/push/status", requireAuth(s.handlePushStatus)},
		{"GET /api/push/config", requireAuth(s.handlePushConfig)},
		{"POST /api/push/subscriptions", requireAuth(s.handlePushSubscribe)},
		{"DELETE /api/push/subscriptions", requireAuth(s.handlePushUnsubscribe)},
		{"POST /api/admin/notifications", requireAdmin(s.handleCreateCampaign)},
		{"GET /api/admin/notifications", requireAdmin(s.handleListCampaigns)},
		{"DELETE /api/admin/notifications/{id}", requireAdmin(s.handleCancelCampaign)},

		// Account.
		{"GET /api/me", requireAuth(s.handleMe)},
		{"GET /api/settings", requireAuth(s.handleSettings)},
		{"GET /api/recommendations", requireAuth(s.handleRecommendations)},
		{"POST /api/me/listens", requireAuth(s.handleQualifiedListen)},
		{"GET /api/queue", requireAuth(s.handleGetQueue)},
		{"PUT /api/queue", requireAuth(s.handleSaveQueue)},

		// Ads.
		{"GET /api/ads", requireAuth(s.handleAds)},
		{"GET /api/admin/ads", requireAdmin(s.handleAdminAds)},
		{"PUT /api/admin/ads", requireAdmin(s.handleSaveAds)},

		// Catalogue.
		{"GET /api/home", requireAuth(s.handleHome)},
		{"GET /api/search", requireAuth(s.handleSearch)},
		{"GET /api/categories", requireAuth(s.handleCategories)},
		{"GET /api/categories/{id}", requireAuth(s.handleCategory)},
		{"GET /api/albums", requireAuth(s.handleAlbums)},
		{"GET /api/albums/{id}", requireAuth(s.handleAlbum)},
		{"GET /api/artists", requireAuth(s.handleArtists)},
		{"GET /api/artists/{id}", requireAuth(s.handleArtist)},
		{"GET /api/songs", requireAuth(s.handleSongs)},
		{"GET /api/songs/{id}", requireAuth(s.handleSong)},

		// Playlists (owner-scoped).
		{"GET /api/playlists", requireAuth(s.handlePlaylists)},
		{"POST /api/playlists", requireAuth(s.handleCreatePlaylist)},
		{"GET /api/playlists/{id}", requireAuth(s.handlePlaylist)},
		{"PUT /api/playlists/{id}", requireAuth(s.handleUpdatePlaylist)},
		{"DELETE /api/playlists/{id}", requireAuth(s.handleDeletePlaylist)},
		{"POST /api/playlists/{id}/tracks", requireAuth(s.handleAddPlaylistTracks)},
		{"DELETE /api/playlists/{id}/tracks/{entryId}", requireAuth(s.handleRemovePlaylistTrack)},
		{"PUT /api/playlists/{id}/tracks", requireAuth(s.handleReorderPlaylistTracks)},

		// Likes, history, downloads (owner-scoped).
		{"GET /api/me/liked", requireAuth(s.handleLiked)},
		{"PUT /api/me/liked/{id}", requireAuth(s.handleLikeSong)},
		{"DELETE /api/me/liked/{id}", requireAuth(s.handleUnlikeSong)},
		{"GET /api/me/liked/albums", requireAuth(s.handleLikedAlbums)},
		{"PUT /api/me/liked/albums/{id}", requireAuth(s.handleLikeAlbum)},
		{"DELETE /api/me/liked/albums/{id}", requireAuth(s.handleUnlikeAlbum)},
		{"GET /api/me/history", requireAuth(s.handleHistory)},
		{"POST /api/me/history/{id}", requireAuth(s.handleRegisterPlay)},
		{"GET /api/me/downloads", requireAuth(s.handleDownloads)},
		{"POST /api/songs/{id}/download", requireAuth(s.handleDownload)},

		// Media.
		{"GET /api/artwork/{id}", requireAuth(s.handleArtwork)},
		{"GET /api/stream/{id}", requireAuth(s.handleStream)},

		// API keys.
		{"GET /api/admin/api-keys", requireAdmin(s.handleListAPIKeys)},
		{"POST /api/admin/api-keys", requireAdmin(s.handleCreateAPIKey)},
		{"DELETE /api/admin/api-keys/{id}", requireAdmin(s.handleRevokeAPIKey)},

		// Admin users.
		{"GET /api/admin/users", requireAdmin(s.handleAdminListUsers)},
		{"POST /api/admin/users", requireAdmin(s.handleAdminCreateUser)},
		{"PUT /api/admin/users/{id}", requireAdmin(s.handleAdminUpdateUser)},
		{"DELETE /api/admin/users/{id}", requireAdmin(s.handleAdminDeleteUser)},

		// Admin catalogue. The album and artist listings are the public ones;
		// only the admin route needs the admin credential.
		{"GET /api/admin/categories", requireAdmin(s.handleCategories)},
		{"POST /api/admin/categories", requireAdmin(s.handleAdminCreateCategory)},
		{"GET /api/admin/categories/{id}", requireAdmin(s.handleAdminCategoryDetail)},
		{"PUT /api/admin/categories/{id}", requireAdmin(s.handleAdminUpdateCategory)},
		{"DELETE /api/admin/categories/{id}", requireAdmin(s.handleAdminDeleteCategory)},
		{"GET /api/admin/albums", requireAdmin(s.handleAlbums)},
		{"GET /api/admin/artists", requireAdmin(s.handleArtists)},
		{"GET /api/admin/songs", requireAdmin(s.handleAdminSongs)},
		{"POST /api/admin/songs", requireAdmin(s.handleAdminUploadSong)},
		{"POST /api/admin/albums/{id}/photo", requireAdmin(s.uploadPhoto(store.EntityAlbum))},
		{"DELETE /api/admin/albums/{id}/photo", requireAdmin(s.deletePhoto(store.EntityAlbum))},
		{"POST /api/admin/songs/{id}/photo", requireAdmin(s.uploadPhoto(store.EntitySong))},
		{"DELETE /api/admin/songs/{id}/photo", requireAdmin(s.deletePhoto(store.EntitySong))},
		{"POST /api/admin/categories/{id}/photo", requireAdmin(s.uploadPhoto(store.EntityCategory))},
		{"DELETE /api/admin/categories/{id}/photo", requireAdmin(s.deletePhoto(store.EntityCategory))},
		{"POST /api/scan", requireAdmin(s.handleScan)},
	}
}

// Handler builds the HTTP handler with all routes and middleware.
func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	for _, rt := range s.routes() {
		mux.Handle(rt.Pattern, rt.Handler)
	}
	// Static UI: everything the API did not claim.
	mux.Handle("/", s.handleStatic())
	return s.middleware(mux)
}

// authenticate resolves the bearer token of a request. ok is false when the
// response is already written (missing/invalid token); adminOnly turns a
// valid non-admin token into a 403.
func (s *Server) authenticate(w http.ResponseWriter, r *http.Request, adminOnly bool) (*auth.Claims, bool) {
	token := s.auth.TokenFromRequest(r)
	if token == "" {
		writeError(w, http.StatusUnauthorized, "Não autenticado")
		return nil, false
	}
	claims, err := s.auth.Parse(token)
	if err != nil {
		writeError(w, http.StatusUnauthorized, "Não autenticado")
		return nil, false
	}
	if adminOnly && !claims.IsAdmin {
		writeError(w, http.StatusForbidden, "Sem permissão")
		return nil, false
	}
	if s.auth.NeedsRefresh(claims) {
		if fresh, err := s.auth.Sign(modelUserFromClaims(claims)); err == nil {
			w.Header().Set(auth.HeaderName, fresh)
		}
	}
	return claims, true
}

// requireAuth validates the JWT (header or ?jwt=), stores the user in the
// context and refreshes the token when past half its lifetime.
func (s *Server) requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		claims, ok := s.authenticate(w, r, false)
		if !ok {
			return
		}
		if !claims.IsAdmin && !s.pushAccessAllowed(w, r, claims.UserID) {
			return
		}
		ctx := context.WithValue(r.Context(), ctxUserID, claims.UserID)
		next(w, r.WithContext(ctx))
	}
}

func (s *Server) requireAdmin(next http.HandlerFunc) http.HandlerFunc {
	apiKey := s.requireAdminAPIKey(next)
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get(apiKeyHeader) != "" {
			apiKey(w, r)
			return
		}
		claims, ok := s.authenticate(w, r, true)
		if !ok {
			return
		}
		ctx := context.WithValue(r.Context(), ctxUserID, claims.UserID)
		next(w, r.WithContext(ctx))
	}
}

func (s *Server) middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()

		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", strings.Join([]string{
			"Content-Type", auth.HeaderName, "Authorization", apiKeyHeader, pushUnavailableHeader,
		}, ", "))
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
