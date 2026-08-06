package server

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"play-music/internal/model"
	"play-music/internal/phone"
	"play-music/internal/store"
	"play-music/internal/version"
)

// userFromContext resolves the full user record for the request.
func (s *Server) userFromContext(ctx context.Context) (*model.User, error) {
	return s.store.GetUser(ctx, userIDOf(ctx))
}

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	u, err := s.userFromContext(r.Context())
	if err != nil {
		handleStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, u)
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
	ctx := r.Context()
	userID := userIDOf(ctx)
	filter := s.filterUser(r)

	var sections []model.HomeSection
	if userID != "" && !isAdminOf(ctx) {
		// Client: one section per granted category.
		cats, err := s.store.GrantedCategoryIDs(ctx, userID)
		if err != nil {
			handleStoreError(w, err)
			return
		}
		for _, cid := range cats {
			cat, err := s.store.GetCategory(ctx, cid)
			if err != nil {
				continue
			}
			albums, err := s.store.CategoryAlbums(ctx, cid)
			if err != nil {
				handleStoreError(w, err)
				return
			}
			if len(albums) > 0 {
				sections = append(sections, model.HomeSection{Title: cat.Name, Albums: albums})
			}
		}
	} else {
		// Admin: recently added, most played, liked.
		recent, err := s.store.RecentlyAddedAlbums(ctx, filter, 12)
		if err != nil {
			handleStoreError(w, err)
			return
		}
		top, err := s.store.MostPlayedAlbums(ctx, filter, 12)
		if err != nil {
			handleStoreError(w, err)
			return
		}
		liked, err := s.store.LikedAlbums(ctx, filter, 12)
		if err != nil {
			handleStoreError(w, err)
			return
		}
		if len(recent) > 0 {
			sections = append(sections, model.HomeSection{Title: "Adicionados recentemente", Albums: recent})
		}
		if len(top) > 0 {
			sections = append(sections, model.HomeSection{Title: "Mais ouvidas", Albums: top})
		}
		if len(liked) > 0 {
			sections = append(sections, model.HomeSection{Title: "Álbuns curtidos", Albums: liked})
		}
	}
	genres, err := s.store.Genres(ctx, filter, 20)
	if err != nil {
		handleStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, model.Home{Sections: sections, Genres: genres})
}

func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	ctx := r.Context()
	userID := userIDOf(ctx)
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
	if res.Songs, err = s.store.SearchSongs(ctx, userID, q, 30); err != nil {
		handleStoreError(w, err)
		return
	}
	if res.Albums, err = s.store.SearchAlbums(ctx, userID, q, 20); err != nil {
		handleStoreError(w, err)
		return
	}
	if res.Artists, err = s.store.SearchArtists(ctx, userID, q, 20); err != nil {
		handleStoreError(w, err)
		return
	}
	if res.Playlists, err = s.store.SearchPlaylists(ctx, userID, q, 20); err != nil {
		handleStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

// ---------- categories (client view) ----------

func (s *Server) handleCategories(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID := userIDOf(ctx)
	var cats []model.Category
	var err error
	if userID != "" && !isAdminOf(ctx) {
		cats, err = s.store.UserCategories(ctx, userID)
	} else {
		cats, err = s.store.GetCategories(ctx)
	}
	if err != nil {
		handleStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, cats)
}

// ---------- albums ----------

func (s *Server) handleAlbums(w http.ResponseWriter, r *http.Request) {
	albums, err := s.store.GetAlbums(r.Context(), s.filterUser(r))
	if err != nil {
		handleStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, albums)
}

func (s *Server) handleAlbum(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := parseID(r)
	ok, err := s.store.CanAccessAlbum(ctx, s.filterUser(r), id)
	if err != nil {
		handleStoreError(w, err)
		return
	}
	if !ok {
		writeError(w, http.StatusForbidden, "Sem permissão")
		return
	}
	album, err := s.store.GetAlbum(ctx, id)
	if err != nil {
		handleStoreError(w, err)
		return
	}
	songs, err := s.store.SongsByAlbum(ctx, album.ID)
	if err != nil {
		handleStoreError(w, err)
		return
	}
	album.Songs = songs
	writeJSON(w, http.StatusOK, album)
}

// ---------- artists ----------

func (s *Server) handleArtists(w http.ResponseWriter, r *http.Request) {
	artists, err := s.store.GetArtists(r.Context(), s.filterUser(r))
	if err != nil {
		handleStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, artists)
}

func (s *Server) handleArtist(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	artist, err := s.store.GetArtist(ctx, parseID(r))
	if err != nil {
		handleStoreError(w, err)
		return
	}
	albums, err := s.store.AlbumsByArtist(ctx, s.filterUser(r), artist.ID)
	if err != nil {
		handleStoreError(w, err)
		return
	}
	if len(albums) == 0 {
		writeError(w, http.StatusForbidden, "Sem permissão")
		return
	}
	top, err := s.store.TopSongsByArtist(ctx, s.filterUser(r), artist.ID, 10)
	if err != nil {
		handleStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"id":         artist.ID,
		"name":       artist.Name,
		"albumCount": len(albums),
		"songCount":  artist.SongCount,
		"liked":      artist.Liked,
		"topSongs":   top,
		"albums":     albums,
	})
}

// ---------- songs ----------

func (s *Server) handleSong(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := parseID(r)
	ok, err := s.store.CanAccessSong(ctx, s.filterUser(r), id)
	if err != nil {
		handleStoreError(w, err)
		return
	}
	if !ok {
		writeError(w, http.StatusForbidden, "Sem permissão")
		return
	}
	song, err := s.store.GetSong(ctx, id)
	if err != nil {
		handleStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, song)
}

// ---------- playlists (owner-scoped) ----------

func (s *Server) handlePlaylists(w http.ResponseWriter, r *http.Request) {
	playlists, err := s.store.GetPlaylists(r.Context(), userIDOf(r.Context()))
	if err != nil {
		handleStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, playlists)
}

func (s *Server) handlePlaylist(w http.ResponseWriter, r *http.Request) {
	pl, err := s.store.GetPlaylist(r.Context(), userIDOf(r.Context()), parseID(r))
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
	pl, err := s.store.CreatePlaylist(r.Context(), userIDOf(r.Context()), strings.TrimSpace(req.Name), req.Comment, req.SongIDs)
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
	if err := s.store.UpdatePlaylist(r.Context(), userIDOf(r.Context()), parseID(r), patch); err != nil {
		handleStoreError(w, err)
		return
	}
	writeNoContent(w)
}

func (s *Server) handleDeletePlaylist(w http.ResponseWriter, r *http.Request) {
	if err := s.store.DeletePlaylist(r.Context(), userIDOf(r.Context()), parseID(r)); err != nil {
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
	if err := s.store.AddPlaylistTracks(r.Context(), userIDOf(r.Context()), parseID(r), req.SongIDs); err != nil {
		handleStoreError(w, err)
		return
	}
	writeNoContent(w)
}

func (s *Server) handleRemovePlaylistTrack(w http.ResponseWriter, r *http.Request) {
	entryID := r.PathValue("entryId")
	if err := s.store.RemovePlaylistTrack(r.Context(), userIDOf(r.Context()), parseID(r), entryID); err != nil {
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
	if err := s.store.ReorderPlaylistTracks(r.Context(), userIDOf(r.Context()), parseID(r), req.From, req.To); err != nil {
		handleStoreError(w, err)
		return
	}
	writeNoContent(w)
}

// ---------- liked ----------

func (s *Server) handleLiked(w http.ResponseWriter, r *http.Request) {
	songs, err := s.store.LikedSongs(r.Context(), userIDOf(r.Context()), 100)
	if err != nil {
		handleStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, songs)
}

func (s *Server) handleLike(w http.ResponseWriter, r *http.Request) {
	if err := s.store.SetLike(r.Context(), userIDOf(r.Context()), "song", parseID(r), true); err != nil {
		handleStoreError(w, err)
		return
	}
	writeNoContent(w)
}

func (s *Server) handleUnlike(w http.ResponseWriter, r *http.Request) {
	if err := s.store.SetLike(r.Context(), userIDOf(r.Context()), "song", parseID(r), false); err != nil {
		handleStoreError(w, err)
		return
	}
	writeNoContent(w)
}

// ---------- history ----------

func (s *Server) handleHistory(w http.ResponseWriter, r *http.Request) {
	songs, err := s.store.HistorySongs(r.Context(), userIDOf(r.Context()), 50)
	if err != nil {
		handleStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, songs)
}

func (s *Server) handleRegisterPlay(w http.ResponseWriter, r *http.Request) {
	if err := s.store.RegisterPlay(r.Context(), userIDOf(r.Context()), parseID(r)); err != nil {
		handleStoreError(w, err)
		return
	}
	writeNoContent(w)
}

// ---------- queue ----------

func (s *Server) handleGetQueue(w http.ResponseWriter, r *http.Request) {
	data, err := s.store.GetPlayQueue(r.Context(), userIDOf(r.Context()))
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
	if err := s.store.SavePlayQueue(r.Context(), userIDOf(r.Context()), data); err != nil {
		handleStoreError(w, err)
		return
	}
	writeNoContent(w)
}

// ---------- lyrics ----------

func (s *Server) handleLyrics(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := parseID(r)
	ok, err := s.store.CanAccessSong(ctx, s.filterUser(r), id)
	if err != nil {
		handleStoreError(w, err)
		return
	}
	if !ok {
		writeError(w, http.StatusForbidden, "Sem permissão")
		return
	}
	lyrics, err := s.lyrics.Lookup(ctx, id)
	if err != nil {
		handleStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, lyrics)
}

// ---------- scan (manual trigger, admin) ----------

func (s *Server) handleScan(w http.ResponseWriter, r *http.Request) {
	go s.scanner.Run(context.Background())
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "scan iniciado"})
}

// ---------- admin: users ----------

func (s *Server) handleAdminListUsers(w http.ResponseWriter, r *http.Request) {
	users, err := s.store.ListUsers(r.Context())
	if err != nil {
		handleStoreError(w, err)
		return
	}
	for i := range users {
		users[i].Phone = phone.Format(users[i].Phone)
	}
	writeJSON(w, http.StatusOK, users)
}

type adminUserRequest struct {
	Name        string   `json:"name"`
	Username    string   `json:"username"`
	Phone       string   `json:"phone"`
	Password    string   `json:"password"`
	CategoryIDs []string `json:"categoryIds"`
}

func (s *Server) handleAdminCreateUser(w http.ResponseWriter, r *http.Request) {
	var req adminUserRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Requisição inválida")
		return
	}
	if strings.TrimSpace(req.Phone) == "" {
		writeError(w, http.StatusBadRequest, "Telefone é obrigatório")
		return
	}
	if req.Password == "" {
		writeError(w, http.StatusBadRequest, "Senha é obrigatória")
		return
	}
	normalized, err := phone.Normalize(req.Phone)
	if err != nil {
		writeError(w, http.StatusBadRequest, phone.ErrInvalid.Error())
		return
	}
	hash, err := bcryptPassword(req.Password)
	if err != nil {
		handleStoreError(w, err)
		return
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		name = normalized
	}
	u := &model.User{ID: store.NewID(), Phone: normalized, Name: name, IsAdmin: false}
	if err := s.store.CreateUser(r.Context(), u, hash, req.CategoryIDs); err != nil {
		handleStoreError(w, err)
		return
	}
	u.Phone = phone.Format(u.Phone)
	writeJSON(w, http.StatusCreated, u)
}

func (s *Server) handleAdminUpdateUser(w http.ResponseWriter, r *http.Request) {
	var req adminUserRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Requisição inválida")
		return
	}
	id := r.PathValue("id")
	ctx := r.Context()
	// Cannot edit yourself (avoid removing the only admin).
	if id == userIDOf(ctx) {
		writeError(w, http.StatusBadRequest, "Não é possível editar a própria conta aqui")
		return
	}
	if _, err := s.store.GetUser(ctx, id); err != nil {
		handleStoreError(w, err)
		return
	}
	patch := store.UserPatch{
		Name:          strings.TrimSpace(req.Name),
		PasswordHash:  "",
		SetCategories: req.CategoryIDs != nil,
		CategoryIDs:   req.CategoryIDs,
	}
	if req.Password != "" {
		hash, err := bcryptPassword(req.Password)
		if err != nil {
			handleStoreError(w, err)
			return
		}
		patch.PasswordHash = hash
	}
	if req.Phone != "" {
		normalized, err := phone.Normalize(req.Phone)
		if err != nil {
			writeError(w, http.StatusBadRequest, phone.ErrInvalid.Error())
			return
		}
		patch.Phone = normalized
	}
	if err := s.store.UpdateUser(ctx, id, patch); err != nil {
		handleStoreError(w, err)
		return
	}
	writeNoContent(w)
}

func (s *Server) handleAdminDeleteUser(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if id == userIDOf(r.Context()) {
		writeError(w, http.StatusBadRequest, "Não é possível excluir a própria conta")
		return
	}
	if err := s.store.DeleteUser(r.Context(), id); err != nil {
		handleStoreError(w, err)
		return
	}
	writeNoContent(w)
}

// ---------- admin: categories ----------

func (s *Server) handleAdminListCategories(w http.ResponseWriter, r *http.Request) {
	cats, err := s.store.GetCategories(r.Context())
	if err != nil {
		handleStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, cats)
}

func (s *Server) handleAdminCreateCategory(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Requisição inválida")
		return
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		writeError(w, http.StatusBadRequest, "Nome da categoria é obrigatório")
		return
	}
	cat, err := s.store.CreateCategory(r.Context(), name)
	if err != nil {
		handleStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, cat)
}

func (s *Server) handleAdminCategoryDetail(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	albumIDs, artistIDs, err := s.store.CategoryDetail(r.Context(), id)
	if err != nil {
		handleStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"id":        id,
		"albumIds":  albumIDs,
		"artistIds": artistIDs,
	})
}

func (s *Server) handleAdminUpdateCategory(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name      string   `json:"name"`
		AlbumIDs  []string `json:"albumIds"`
		ArtistIDs []string `json:"artistIds"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Requisição inválida")
		return
	}
	if err := s.store.UpdateCategory(r.Context(), r.PathValue("id"),
		strings.TrimSpace(req.Name), req.AlbumIDs, req.ArtistIDs); err != nil {
		handleStoreError(w, err)
		return
	}
	writeNoContent(w)
}

func (s *Server) handleAdminDeleteCategory(w http.ResponseWriter, r *http.Request) {
	if err := s.store.DeleteCategory(r.Context(), r.PathValue("id")); err != nil {
		handleStoreError(w, err)
		return
	}
	writeNoContent(w)
}

// ---------- admin: albums / artists (assignment) ----------

func (s *Server) handleAdminAlbums(w http.ResponseWriter, r *http.Request) {
	albums, err := s.store.GetAlbums(r.Context(), "")
	if err != nil {
		handleStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, albums)
}

func (s *Server) handleAdminArtists(w http.ResponseWriter, r *http.Request) {
	artists, err := s.store.GetArtists(r.Context(), "")
	if err != nil {
		handleStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, artists)
}

// ---------- admin: album photo ----------

const maxPhotoBytes = 15 << 20 // 15MB

func (s *Server) handleAdminUploadPhoto(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxPhotoBytes)
	if err := r.ParseMultipartForm(maxPhotoBytes); err != nil {
		writeError(w, http.StatusBadRequest, "Arquivo muito grande (máx 15MB) ou inválido")
		return
	}
	file, _, err := r.FormFile("photo")
	if err != nil {
		writeError(w, http.StatusBadRequest, "Campo 'photo' obrigatório")
		return
	}
	defer file.Close()
	data, err := io.ReadAll(io.LimitReader(file, maxPhotoBytes))
	if err != nil {
		writeError(w, http.StatusBadRequest, "Falha ao ler arquivo")
		return
	}
	albumID := r.PathValue("id")
	if ok, err := s.store.AlbumExists(r.Context(), albumID); err != nil || !ok {
		writeError(w, http.StatusNotFound, "Álbum não encontrado")
		return
	}
	if err := s.artwork.UploadAlbumPhoto(r.Context(), albumID, data); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeNoContent(w)
}

func (s *Server) handleAdminDeletePhoto(w http.ResponseWriter, r *http.Request) {
	if err := s.artwork.DeleteAlbumPhoto(r.Context(), r.PathValue("id")); err != nil {
		handleStoreError(w, err)
		return
	}
	writeNoContent(w)
}
