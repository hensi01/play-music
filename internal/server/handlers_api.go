package server

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"

	"play-music/internal/model"
	"play-music/internal/store"
	"play-music/internal/version"
)

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.auth.AdminUser())
}

func (s *Server) handleSettings(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, model.Settings{
		AppName:     "Play Music",
		Version:     version.Version,
		LibraryName: "Play Music",
		MusicFolder: s.cfg.MusicFolder,
	})
}

func (s *Server) handleHome(w http.ResponseWriter, r *http.Request) {
	recent, err := s.store.RecentlyAddedAlbums(r.Context(), 12)
	if err != nil {
		handleStoreError(w, err)
		return
	}
	top, err := s.store.MostPlayedAlbums(r.Context(), 12)
	if err != nil {
		handleStoreError(w, err)
		return
	}
	liked, err := s.store.LikedAlbums(r.Context(), 12)
	if err != nil {
		handleStoreError(w, err)
		return
	}
	genres, err := s.store.Genres(r.Context(), 20)
	if err != nil {
		handleStoreError(w, err)
		return
	}
	sections := []model.HomeSection{}
	if len(recent) > 0 {
		sections = append(sections, model.HomeSection{Title: "Adicionados recentemente", Albums: recent})
	}
	if len(top) > 0 {
		sections = append(sections, model.HomeSection{Title: "Mais ouvidas", Albums: top})
	}
	if len(liked) > 0 {
		sections = append(sections, model.HomeSection{Title: "Álbuns curtidos", Albums: liked})
	}
	writeJSON(w, http.StatusOK, model.Home{Sections: sections, Genres: genres})
}

func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	ctx := r.Context()
	res := model.SearchResults{
		Songs:     []model.Song{},
		Albums:    []model.Album{},
		Artists:   []model.Artist{},
		Playlists: []model.Playlist{},
	}
	if q == "" {
		writeJSON(w, http.StatusOK, res)
		return
	}
	var err error
	if res.Songs, err = s.store.SearchSongs(ctx, q, 30); err != nil {
		handleStoreError(w, err)
		return
	}
	if res.Albums, err = s.store.SearchAlbums(ctx, q, 20); err != nil {
		handleStoreError(w, err)
		return
	}
	if res.Artists, err = s.store.SearchArtists(ctx, q, 20); err != nil {
		handleStoreError(w, err)
		return
	}
	if res.Playlists, err = s.store.SearchPlaylists(ctx, q, 20); err != nil {
		handleStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

// ---------- albums ----------

func (s *Server) handleAlbums(w http.ResponseWriter, r *http.Request) {
	albums, err := s.store.GetAlbums(r.Context())
	if err != nil {
		handleStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, albums)
}

func (s *Server) handleAlbum(w http.ResponseWriter, r *http.Request) {
	album, err := s.store.GetAlbum(r.Context(), parseID(r))
	if err != nil {
		handleStoreError(w, err)
		return
	}
	songs, err := s.store.SongsByAlbum(r.Context(), album.ID)
	if err != nil {
		handleStoreError(w, err)
		return
	}
	album.Songs = songs
	writeJSON(w, http.StatusOK, album)
}

// ---------- artists ----------

func (s *Server) handleArtists(w http.ResponseWriter, r *http.Request) {
	artists, err := s.store.GetArtists(r.Context())
	if err != nil {
		handleStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, artists)
}

func (s *Server) handleArtist(w http.ResponseWriter, r *http.Request) {
	artist, err := s.store.GetArtist(r.Context(), parseID(r))
	if err != nil {
		handleStoreError(w, err)
		return
	}
	top, err := s.store.TopSongsByArtist(r.Context(), artist.ID, 10)
	if err != nil {
		handleStoreError(w, err)
		return
	}
	albums, err := s.store.AlbumsByArtist(r.Context(), artist.ID)
	if err != nil {
		handleStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"id":         artist.ID,
		"name":       artist.Name,
		"albumCount": artist.AlbumCount,
		"songCount":  artist.SongCount,
		"liked":      artist.Liked,
		"topSongs":   top,
		"albums":     albums,
	})
}

// ---------- songs ----------

func (s *Server) handleSong(w http.ResponseWriter, r *http.Request) {
	song, err := s.store.GetSong(r.Context(), parseID(r))
	if err != nil {
		handleStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, song)
}

// ---------- playlists ----------

func (s *Server) handlePlaylists(w http.ResponseWriter, r *http.Request) {
	playlists, err := s.store.GetPlaylists(r.Context())
	if err != nil {
		handleStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, playlists)
}

func (s *Server) handlePlaylist(w http.ResponseWriter, r *http.Request) {
	pl, err := s.store.GetPlaylist(r.Context(), parseID(r))
	if err != nil {
		handleStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, pl)
}

func (s *Server) handleCreatePlaylist(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name    string   `json:"name"`
		Comment string   `json:"comment"`
		SongIDs []string `json:"songIds"`
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, 1<<20))
	if err != nil {
		writeError(w, http.StatusBadRequest, "Requisição inválida")
		return
	}
	if err := json.Unmarshal(body, &req); err != nil {
		writeError(w, http.StatusBadRequest, "Requisição inválida")
		return
	}
	if strings.TrimSpace(req.Name) == "" {
		writeError(w, http.StatusBadRequest, "Nome da playlist é obrigatório")
		return
	}
	pl, err := s.store.CreatePlaylist(r.Context(), strings.TrimSpace(req.Name), req.Comment, req.SongIDs)
	if err != nil {
		handleStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, pl)
}

func (s *Server) handleUpdatePlaylist(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name    string `json:"name"`
		Comment string `json:"comment"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Requisição inválida")
		return
	}
	patch := &model.Playlist{Name: req.Name, Comment: req.Comment}
	if err := s.store.UpdatePlaylist(r.Context(), parseID(r), patch); err != nil {
		handleStoreError(w, err)
		return
	}
	writeNoContent(w)
}

func (s *Server) handleDeletePlaylist(w http.ResponseWriter, r *http.Request) {
	if err := s.store.DeletePlaylist(r.Context(), parseID(r)); err != nil {
		handleStoreError(w, err)
		return
	}
	writeNoContent(w)
}

func (s *Server) handleAddPlaylistTracks(w http.ResponseWriter, r *http.Request) {
	var req struct {
		SongIDs []string `json:"songIds"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Requisição inválida")
		return
	}
	if err := s.store.AddPlaylistTracks(r.Context(), parseID(r), req.SongIDs); err != nil {
		handleStoreError(w, err)
		return
	}
	writeNoContent(w)
}

func (s *Server) handleRemovePlaylistTrack(w http.ResponseWriter, r *http.Request) {
	entryID := r.PathValue("entryId")
	if err := s.store.RemovePlaylistTrack(r.Context(), parseID(r), entryID); err != nil {
		handleStoreError(w, err)
		return
	}
	writeNoContent(w)
}

func (s *Server) handleReorderPlaylistTracks(w http.ResponseWriter, r *http.Request) {
	var req struct {
		From int `json:"from"`
		To   int `json:"to"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Requisição inválida")
		return
	}
	if err := s.store.ReorderPlaylistTracks(r.Context(), parseID(r), req.From, req.To); err != nil {
		handleStoreError(w, err)
		return
	}
	writeNoContent(w)
}

// ---------- liked ----------

func (s *Server) handleLiked(w http.ResponseWriter, r *http.Request) {
	songs, err := s.store.LikedSongs(r.Context(), 100)
	if err != nil {
		handleStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, songs)
}

func (s *Server) handleLike(w http.ResponseWriter, r *http.Request) {
	if err := s.store.SetLike(r.Context(), "song", parseID(r), true); err != nil {
		handleStoreError(w, err)
		return
	}
	writeNoContent(w)
}

func (s *Server) handleUnlike(w http.ResponseWriter, r *http.Request) {
	if err := s.store.SetLike(r.Context(), "song", parseID(r), false); err != nil {
		handleStoreError(w, err)
		return
	}
	writeNoContent(w)
}

// ---------- history ----------

func (s *Server) handleHistory(w http.ResponseWriter, r *http.Request) {
	songs, err := s.store.HistorySongs(r.Context(), 50)
	if err != nil {
		handleStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, songs)
}

func (s *Server) handleRegisterPlay(w http.ResponseWriter, r *http.Request) {
	if err := s.store.RegisterPlay(r.Context(), parseID(r)); err != nil {
		if errors.Is(err, store.ErrNotFound) {
			writeError(w, http.StatusNotFound, "Não encontrado")
			return
		}
		handleStoreError(w, err)
		return
	}
	writeNoContent(w)
}

// ---------- queue ----------

func (s *Server) handleGetQueue(w http.ResponseWriter, r *http.Request) {
	data, err := s.store.GetPlayQueue(r.Context())
	if err != nil {
		handleStoreError(w, err)
		return
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Write(data)
}

func (s *Server) handleSaveQueue(w http.ResponseWriter, r *http.Request) {
	data, err := io.ReadAll(io.LimitReader(r.Body, 16<<20))
	if err != nil {
		writeError(w, http.StatusBadRequest, "Requisição inválida")
		return
	}
	if err := s.store.SavePlayQueue(r.Context(), data); err != nil {
		handleStoreError(w, err)
		return
	}
	writeNoContent(w)
}

// ---------- lyrics ----------

func (s *Server) handleLyrics(w http.ResponseWriter, r *http.Request) {
	lyrics, err := s.lyrics.Lookup(r.Context(), parseID(r))
	if err != nil {
		handleStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, lyrics)
}

// ---------- scan (manual trigger) ----------

func (s *Server) handleScan(w http.ResponseWriter, r *http.Request) {
	go s.scanner.Run(context.Background())
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "scan iniciado"})
}
