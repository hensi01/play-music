package server

import (
	"log/slog"
	"net/http"
	"time"

	"play-music/internal/artwork"
	"play-music/internal/auth"
	"play-music/internal/config"
	"play-music/internal/lyrics"
	"play-music/internal/scanner"
	"play-music/internal/store"
	"play-music/internal/stream"
)

type Server struct {
	cfg     *config.Config
	store   *store.Store
	auth    *auth.Auth
	stream  *stream.Service
	artwork *artwork.Service
	lyrics  *lyrics.Service
	scanner *scanner.Scanner
	log     *slog.Logger
}

type Dependencies struct {
	Config  *config.Config
	Store   *store.Store
	Auth    *auth.Auth
	Stream  *stream.Service
	Artwork *artwork.Service
	Lyrics  *lyrics.Service
	Scanner *scanner.Scanner
	Log     *slog.Logger
}

func New(deps Dependencies) *Server {
	return &Server{
		cfg:     deps.Config,
		store:   deps.Store,
		auth:    deps.Auth,
		stream:  deps.Stream,
		artwork: deps.Artwork,
		lyrics:  deps.Lyrics,
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
	mux.HandleFunc("POST /auth/createAdmin", s.handleLogin)

	// API (JWT required).
	mux.Handle("GET /api/me", s.requireAuth(http.HandlerFunc(s.handleMe)))
	mux.Handle("GET /api/settings", s.requireAuth(http.HandlerFunc(s.handleSettings)))
	mux.Handle("GET /api/home", s.requireAuth(http.HandlerFunc(s.handleHome)))
	mux.Handle("GET /api/search", s.requireAuth(http.HandlerFunc(s.handleSearch)))

	mux.Handle("GET /api/albums", s.requireAuth(http.HandlerFunc(s.handleAlbums)))
	mux.Handle("GET /api/albums/{id}", s.requireAuth(http.HandlerFunc(s.handleAlbum)))
	mux.Handle("GET /api/artists", s.requireAuth(http.HandlerFunc(s.handleArtists)))
	mux.Handle("GET /api/artists/{id}", s.requireAuth(http.HandlerFunc(s.handleArtist)))
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
	mux.Handle("GET /api/lyrics/{id}", s.requireAuth(http.HandlerFunc(s.handleLyrics)))

	// Media (JWT via header or ?jwt= query).
	mux.Handle("GET /api/artwork/{id}", s.requireAuth(http.HandlerFunc(s.handleArtwork)))
	mux.Handle("GET /api/stream/{id}", s.requireAuth(http.HandlerFunc(s.handleStream)))

	// Manual scan trigger (not used by the UI, handy for testing).
	mux.Handle("POST /api/scan", s.requireAuth(http.HandlerFunc(s.handleScan)))

	// Static UI.
	mux.Handle("/", s.handleStatic())

	return s.middleware(mux)
}

// requireAuth validates the JWT (header or ?jwt=) and refreshes it in the
// X-ND-Authorization response header when past half its lifetime.
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
			if fresh, err := s.auth.Sign(r.Context(), claims.Username); err == nil {
				w.Header().Set(auth.HeaderName, fresh)
			}
		}
		next.ServeHTTP(w, r)
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
