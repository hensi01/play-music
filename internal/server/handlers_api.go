package server

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

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
			songs, err := s.store.CategorySongs(ctx, cid)
			if err != nil {
				handleStoreError(w, err)
				return
			}
			if len(songs) > 0 {
				sections = append(sections, model.HomeSection{Title: cat.Name, Songs: songs})
			}
		}
	} else {
		// Admin: recently added and most played songs.
		recent, err := s.store.RecentlyAddedSongs(ctx, filter, 24)
		if err != nil {
			handleStoreError(w, err)
			return
		}
		top, err := s.store.MostPlayedSongs(ctx, filter, 24)
		if err != nil {
			handleStoreError(w, err)
			return
		}
		if len(recent) > 0 {
			sections = append(sections, model.HomeSection{Title: "Adicionadas recentemente", Songs: recent})
		}
		if len(top) > 0 {
			sections = append(sections, model.HomeSection{Title: "Mais ouvidas", Songs: top})
		}
	}
	writeJSON(w, http.StatusOK, model.Home{Sections: sections, Genres: []model.Genre{}})
}

func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	typ := strings.TrimSpace(r.URL.Query().Get("type"))
	ctx := r.Context()
	userID := userIDOf(ctx)
	res := model.SearchResults{
		Songs:      []model.Song{},
		Albums:     []model.Album{},
		Artists:    []model.Artist{},
		Playlists:  []model.Playlist{},
		Categories: []model.Category{},
	}
	if q == "" {
		writeJSON(w, http.StatusOK, res)
		return
	}
	// type=songs|categories|playlists|all (default all)
	searchAll := typ == "" || typ == "all"
	accessUser := s.filterUser(r) // "" for admin (sees everything)
	var err error
	if searchAll || typ == "songs" {
		if res.Songs, err = s.store.SearchSongs(ctx, accessUser, q, 30); err != nil {
			handleStoreError(w, err)
			return
		}
	}
	if searchAll || typ == "categories" {
		if res.Categories, err = s.store.SearchCategories(ctx, accessUser, q, 20); err != nil {
			handleStoreError(w, err)
			return
		}
	}
	if searchAll || typ == "playlists" {
		if res.Playlists, err = s.store.SearchPlaylists(ctx, userID, q, 20); err != nil {
			handleStoreError(w, err)
			return
		}
	}
	// Legacy album/artist results (unused by the current UI).
	if searchAll {
		if res.Albums, err = s.store.SearchAlbums(ctx, accessUser, q, 20); err != nil {
			handleStoreError(w, err)
			return
		}
		if res.Artists, err = s.store.SearchArtists(ctx, accessUser, q, 20); err != nil {
			handleStoreError(w, err)
			return
		}
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

// handleCategory returns one category with its songs. Clients may only open
// categories granted to them (403 otherwise).
func (s *Server) handleCategory(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := parseID(r)
	if userID := s.filterUser(r); userID != "" {
		ok, err := s.store.CanAccessCategory(ctx, userID, id)
		if err != nil {
			handleStoreError(w, err)
			return
		}
		if !ok {
			writeError(w, http.StatusForbidden, "Sem permissão")
			return
		}
	}
	cat, err := s.store.GetCategory(ctx, id)
	if err != nil {
		handleStoreError(w, err)
		return
	}
	songs, err := s.store.CategorySongs(ctx, id)
	if err != nil {
		handleStoreError(w, err)
		return
	}
	cat.Songs = songs
	writeJSON(w, http.StatusOK, cat)
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

// handleSongs lists every song accessible to the user (Library page).
func (s *Server) handleSongs(w http.ResponseWriter, r *http.Request) {
	songs, err := s.store.AllSongs(r.Context(), s.filterUser(r))
	if err != nil {
		handleStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, songs)
}

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
	pl, err := s.store.GetPlaylist(r.Context(), userIDOf(r.Context()), s.filterUser(r), parseID(r))
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
	pl, err := s.store.CreatePlaylist(r.Context(), userIDOf(r.Context()), s.filterUser(r), strings.TrimSpace(req.Name), req.Comment, req.SongIDs)
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
	if err := s.store.AddPlaylistTracks(r.Context(), userIDOf(r.Context()), s.filterUser(r), parseID(r), req.SongIDs); err != nil {
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
	// Likes are owner-scoped: the real user id is the owner; the access
	// filter uses filterUser so admins ("" ) see everything they liked.
	songs, err := s.store.LikedSongs(r.Context(), userIDOf(r.Context()), s.filterUser(r), 100)
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
	// History is owner-scoped (like likes/playlists): the real user id owns
	// the history rows; the access filter uses filterUser so admins ("")
	// see their whole history.
	songs, err := s.store.HistorySongs(r.Context(), userIDOf(r.Context()), s.filterUser(r), 50)
	if err != nil {
		handleStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, songs)
}

func (s *Server) handleRegisterPlay(w http.ResponseWriter, r *http.Request) {
	// History is owner-scoped (like likes/playlists): must use the real user
	// id even for admins, otherwise plays get recorded under "".
	if err := s.store.RegisterPlay(r.Context(), userIDOf(r.Context()), s.filterUser(r), parseID(r)); err != nil {
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
	Email       string   `json:"email"`
	Phone       string   `json:"phone"`
	Password    string   `json:"password"`
	IsAdmin     bool     `json:"isAdmin"`
	CategoryIDs []string `json:"categoryIds"`
}

func (s *Server) handleAdminCreateUser(w http.ResponseWriter, r *http.Request) {
	var req adminUserRequest
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Requisição inválida")
		return
	}
	ctx := r.Context()
	u := &model.User{Name: strings.TrimSpace(req.Name), IsAdmin: req.IsAdmin}
	var hash string
	var err error

	if req.IsAdmin {
		// Administrador: usuário + senha + e-mail obrigatórios (sem telefone).
		u.Username = strings.TrimSpace(req.Username)
		u.Email = strings.TrimSpace(req.Email)
		if u.Username == "" || u.Email == "" {
			writeError(w, http.StatusBadRequest, "Administrador precisa de usuário e e-mail")
			return
		}
		if req.Password == "" {
			writeError(w, http.StatusBadRequest, "Senha é obrigatória")
			return
		}
		hash, err = bcryptPassword(req.Password)
		if err != nil {
			handleStoreError(w, err)
			return
		}
	} else {
		// Cliente: somente telefone (acesso liberado pelas categorias).
		if strings.TrimSpace(req.Phone) == "" {
			writeError(w, http.StatusBadRequest, "Telefone é obrigatório")
			return
		}
		normalized, err := phone.Normalize(req.Phone)
		if err != nil {
			writeError(w, http.StatusBadRequest, phone.ErrInvalid.Error())
			return
		}
		u.Phone = normalized
		hash, err = bcryptPassword(randomSecret())
		if err != nil {
			handleStoreError(w, err)
			return
		}
	}
	if u.Name == "" {
		if u.Username != "" {
			u.Name = u.Username
		} else {
			u.Name = u.Phone
		}
	}
	u.ID = store.NewID()
	if err := s.store.CreateUser(ctx, u, hash, req.CategoryIDs); err != nil {
		handleStoreError(w, err)
		return
	}
	if u.Phone != "" {
		u.Phone = phone.Format(u.Phone)
	}
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
	current, err := s.store.GetUser(ctx, id)
	if err != nil {
		handleStoreError(w, err)
		return
	}

	patch := store.UserPatch{
		Name:          strings.TrimSpace(req.Name),
		PasswordHash:  "",
		SetCategories: req.CategoryIDs != nil,
		CategoryIDs:   req.CategoryIDs,
	}

	// Guard: never demote the last admin (would lock the app out).
	if current.IsAdmin && !req.IsAdmin {
		adminCount := 0
		if err := s.store.Pool().QueryRow(ctx, "SELECT count(*) FROM users WHERE is_admin").Scan(&adminCount); err != nil {
			handleStoreError(w, err)
			return
		}
		if adminCount <= 1 {
			writeError(w, http.StatusBadRequest, "Não é possível remover o último administrador")
			return
		}
	}

	if req.IsAdmin {
		// Vira/continua admin: usuário + e-mail (senha opcional na edição).
		username := strings.TrimSpace(req.Username)
		email := strings.TrimSpace(req.Email)
		if username == "" {
			username = current.Username
		}
		if email == "" {
			email = current.Email
		}
		if username == "" || email == "" {
			writeError(w, http.StatusBadRequest, "Administrador precisa de usuário e e-mail")
			return
		}
		patch.Username = &username
		patch.Email = &email
		phone := ""
		patch.Phone = &phone // limpa o telefone ao virar admin
	} else {
		// Cliente: telefone obrigatório; limpa usuário/e-mail.
		normalized, err := phone.Normalize(strings.TrimSpace(req.Phone))
		if err != nil {
			writeError(w, http.StatusBadRequest, phone.ErrInvalid.Error())
			return
		}
		patch.Phone = &normalized
		clear := ""
		patch.Username = &clear
		patch.Email = &clear
	}
	isAdmin := req.IsAdmin
	patch.IsAdmin = &isAdmin
	if req.Password != "" {
		hash, err := bcryptPassword(req.Password)
		if err != nil {
			handleStoreError(w, err)
			return
		}
		patch.PasswordHash = hash
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
		Name        string `json:"name"`
		CheckoutURL string `json:"checkoutUrl"`
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
	cat, err := s.store.CreateCategory(r.Context(), name, strings.TrimSpace(req.CheckoutURL))
	if err != nil {
		handleStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, cat)
}

func (s *Server) handleAdminCategoryDetail(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	songIDs, err := s.store.CategoryDetail(r.Context(), id)
	if err != nil {
		handleStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"id":      id,
		"songIds": songIDs,
	})
}

func (s *Server) handleAdminUpdateCategory(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name        string   `json:"name"`
		CheckoutURL *string  `json:"checkoutUrl"`
		SongIDs     []string `json:"songIds"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<20)).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "Requisição inválida")
		return
	}
	if err := s.store.UpdateCategory(r.Context(), r.PathValue("id"),
		strings.TrimSpace(req.Name), req.CheckoutURL, req.SongIDs); err != nil {
		handleStoreError(w, err)
		return
	}
	writeNoContent(w)
}

func (s *Server) handleAdminDeleteCategory(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	// Remove a capa da categoria (MinIO + Postgres) para não deixar órfã.
	if err := s.artwork.DeleteCategoryPhoto(ctx, r.PathValue("id")); err != nil {
		s.log.Warn("category cover cleanup failed", "id", r.PathValue("id"), "err", err)
	}
	if err := s.store.DeleteCategory(ctx, r.PathValue("id")); err != nil {
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

// handleAdminSongs lists every song plus the category ids each one belongs to
// (admin song management screen).
func (s *Server) handleAdminSongs(w http.ResponseWriter, r *http.Request) {
	songs, err := s.store.AllSongs(r.Context(), "")
	if err != nil {
		handleStoreError(w, err)
		return
	}
	cats, err := s.store.CategorySongIDs(r.Context())
	if err != nil {
		handleStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"songs":        songs,
		"categoryIds":  cats,
		"categoryList": mustCategories(r.Context(), s),
	})
}

func mustCategories(ctx context.Context, s *Server) []model.Category {
	cats, err := s.store.GetCategories(ctx)
	if err != nil {
		return []model.Category{}
	}
	return cats
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

// ---------- admin: song photo ----------

func (s *Server) handleAdminUploadSongPhoto(w http.ResponseWriter, r *http.Request) {
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
	songID := r.PathValue("id")
	if ok, err := s.store.SongExists(r.Context(), songID); err != nil || !ok {
		writeError(w, http.StatusNotFound, "Música não encontrada")
		return
	}
	if err := s.artwork.UploadSongPhoto(r.Context(), songID, data); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeNoContent(w)
}

func (s *Server) handleAdminDeleteSongPhoto(w http.ResponseWriter, r *http.Request) {
	if err := s.artwork.DeleteSongPhoto(r.Context(), r.PathValue("id")); err != nil {
		handleStoreError(w, err)
		return
	}
	writeNoContent(w)
}

// ---------- admin: category photo ----------

func (s *Server) handleAdminUploadCategoryPhoto(w http.ResponseWriter, r *http.Request) {
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
	categoryID := r.PathValue("id")
	if _, err := s.store.GetCategory(r.Context(), categoryID); err != nil {
		writeError(w, http.StatusNotFound, "Categoria não encontrada")
		return
	}
	if err := s.artwork.UploadCategoryPhoto(r.Context(), categoryID, data); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	writeNoContent(w)
}

func (s *Server) handleAdminDeleteCategoryPhoto(w http.ResponseWriter, r *http.Request) {
	if err := s.artwork.DeleteCategoryPhoto(r.Context(), r.PathValue("id")); err != nil {
		handleStoreError(w, err)
		return
	}
	writeNoContent(w)
}

// ---------- admin: song upload ----------

const maxUploadBytes = 512 << 20 // 512MB

// waitForObject polls the storage until a freshly uploaded object is readable.
// The MinIO endpoint sits behind a caching proxy that can take a few seconds
// to expose a newly written object (observed ~6s); without this, the
// immediate index fails with "Access Denied".
func (s *Server) waitForObject(ctx context.Context, key string) {
	deadline := time.Now().Add(60 * time.Second)
	for {
		ok, err := s.storage.ObjectExists(ctx, key)
		if err == nil && ok {
			return
		}
		if ctx.Err() != nil || time.Now().After(deadline) {
			return
		}
		select {
		case <-time.After(2 * time.Second):
		case <-ctx.Done():
			return
		}
	}
}

var audioMimeByExt = map[string]string{
	".mp3":  "audio/mpeg",
	".m4a":  "audio/mp4",
	".aac":  "audio/aac",
	".ogg":  "audio/ogg",
	".opus": "audio/ogg",
	".wav":  "audio/wav",
	".flac": "audio/flac",
	".wma":  "audio/x-ms-wma",
	".aiff": "audio/aiff",
	".aif":  "audio/aiff",
	".wv":   "audio/wavpack",
	".ape":  "audio/x-ape",
}

// handleAdminUploadSong accepts a multipart form with:
//   - "song" (file, required) — audio file to add to the library
//   - "title"/"artist" (optional) — metadata overrides
//   - "categoryId" (optional) — category the song is assigned to
//   - "photo" (optional) — song photo (jpg/png/webp)
//
// The file is stored under uploads/ in the bucket and indexed immediately.
func (s *Server) handleAdminUploadSong(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadBytes)
	if err := r.ParseMultipartForm(maxUploadBytes); err != nil {
		writeError(w, http.StatusBadRequest, "Arquivo muito grande (máx 512MB) ou inválido")
		return
	}
	file, header, err := r.FormFile("song")
	if err != nil {
		writeError(w, http.StatusBadRequest, "Campo 'song' obrigatório (arquivo de áudio)")
		return
	}
	defer file.Close()

	ext := strings.ToLower(filepath.Ext(header.Filename))
	contentType := audioMimeByExt[ext]
	if contentType == "" {
		writeError(w, http.StatusBadRequest, "Formato de áudio não suportado: "+ext)
		return
	}

	// Buffer to a temp file so we know the size before Put.
	tmp, err := os.CreateTemp("", "pm-upload-*"+ext)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Falha ao processar arquivo")
		return
	}
	defer os.Remove(tmp.Name())
	size, err := io.Copy(tmp, file)
	if err != nil || size == 0 {
		tmp.Close()
		writeError(w, http.StatusBadRequest, "Falha ao ler arquivo")
		return
	}
	if err := tmp.Close(); err != nil {
		writeError(w, http.StatusInternalServerError, "Falha ao processar arquivo")
		return
	}

	key := "uploads/" + store.NewID() + ext
	in, err := os.Open(tmp.Name())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Falha ao processar arquivo")
		return
	}
	defer in.Close()
	if err := s.storage.Put(r.Context(), key, size, contentType, in); err != nil {
		s.log.Error("upload put", "key", key, "err", err)
		writeError(w, http.StatusInternalServerError, "Falha ao enviar para o armazenamento")
		return
	}

	song, err := s.scanner.IndexFile(r.Context(), key, header.Filename)
	if err != nil {
		s.log.Warn("upload index (retry)", "key", key, "err", err)
		s.waitForObject(r.Context(), key)
		song, err = s.scanner.IndexFile(r.Context(), key, header.Filename)
	}
	if err != nil {
		s.log.Error("upload index", "key", key, "err", err)
		writeError(w, http.StatusInternalServerError, "Falha ao indexar a música")
		return
	}

	title := strings.TrimSpace(r.FormValue("title"))
	artist := strings.TrimSpace(r.FormValue("artist"))
	if title != "" || artist != "" {
		if err := s.store.UpdateSongMeta(r.Context(), song.ID, title, artist); err != nil {
			s.log.Warn("upload meta update", "err", err)
		}
		if fresh, err := s.store.GetSong(r.Context(), song.ID); err == nil {
			song = fresh
		}
	}

	if categoryID := strings.TrimSpace(r.FormValue("categoryId")); categoryID != "" {
		if err := s.store.AddSongToCategory(r.Context(), categoryID, song.ID); err != nil {
			s.log.Warn("upload category assign", "err", err)
		}
	}

	if photo, _, err := r.FormFile("photo"); err == nil {
		data, err := io.ReadAll(io.LimitReader(photo, maxPhotoBytes))
		photo.Close()
		if err == nil && len(data) > 0 {
			if err := s.artwork.UploadSongPhoto(r.Context(), song.ID, data); err != nil {
				s.log.Warn("upload photo", "err", err)
			}
		}
	}

	writeJSON(w, http.StatusCreated, song)
}
