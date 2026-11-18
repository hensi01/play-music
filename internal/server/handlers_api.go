package server

import (
	"context"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"play-music/internal/metadata"
	"play-music/internal/model"
	"play-music/internal/phone"
	"play-music/internal/store"
	"play-music/internal/stream"
	"play-music/internal/version"
)

func (s *Server) handleMe(w http.ResponseWriter, r *http.Request) {
	u, err := s.store.GetUser(r.Context(), userIDOf(r.Context()))
	if err != nil {
		handleStoreError(w, err)
		return
	}
	formatPhone(u)
	writeJSON(w, http.StatusOK, u)
}

// appName is the product name served to the UI. It appears twice in the
// settings payload because the two fields serve different screens.
const appName = "Mi Ritmo"

func (s *Server) handleSettings(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, model.Settings{
		AppName:         appName,
		Version:         version.Version,
		LibraryName:     appName,
		AudioExtensions: stream.AudioExtensions(),
	})
}

func (s *Server) handleHome(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	recent, err := s.store.RecentlyAddedSongs(ctx, 24)
	if err != nil {
		handleStoreError(w, err)
		return
	}
	top, err := s.store.MostPlayedSongs(ctx, 24)
	if err != nil {
		handleStoreError(w, err)
		return
	}
	if !s.addLikes(w, r, recent) || !s.addLikes(w, r, top) {
		return
	}
	// An empty section list must marshal as [] rather than null: the published
	// schema declares an array, and a library with nothing recent still has a
	// home screen.
	sections := []model.HomeSection{}
	if len(recent) > 0 {
		sections = append(sections, model.HomeSection{Title: "Adicionadas recentemente", Songs: recent})
	}
	if len(top) > 0 {
		sections = append(sections, model.HomeSection{Title: "Mais ouvidas", Songs: top})
	}
	writeJSON(w, http.StatusOK, model.Home{Sections: sections, Genres: []model.Genre{}})
}

func (s *Server) handleSearch(w http.ResponseWriter, r *http.Request) {
	q := strings.TrimSpace(r.URL.Query().Get("q"))
	typ := strings.TrimSpace(r.URL.Query().Get("type"))
	ctx := r.Context()
	// Every field starts as an empty slice: a type-filtered search leaves the
	// other kinds untouched, and the published schema says array, not null.
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
	// type=songs|albums|artists|categories|playlists|all (default all)
	searchAll := typ == "" || typ == "all"
	var err error
	if searchAll || typ == "songs" {
		if res.Songs, err = s.store.SearchSongs(ctx, q, 30); err != nil {
			handleStoreError(w, err)
			return
		}
	}
	if searchAll || typ == "categories" {
		if res.Categories, err = s.store.SearchCategories(ctx, q, 20); err != nil {
			handleStoreError(w, err)
			return
		}
	}
	if searchAll || typ == "playlists" {
		if res.Playlists, err = s.store.SearchPlaylists(ctx, userIDOf(ctx), q, 20); err != nil {
			handleStoreError(w, err)
			return
		}
	}
	if searchAll || typ == "albums" {
		if res.Albums, err = s.store.SearchAlbums(ctx, q, 20); err != nil {
			handleStoreError(w, err)
			return
		}
	}
	// Artists are only returned by the combined search, for compatibility with
	// clients that expect them in the "all" payload.
	if searchAll {
		if res.Artists, err = s.store.SearchArtists(ctx, q, 20); err != nil {
			handleStoreError(w, err)
			return
		}
		if err = s.store.ApplyArtistLikes(ctx, userIDOf(ctx), res.Artists); err != nil {
			handleStoreError(w, err)
			return
		}
	}
	if err = s.store.ApplyAlbumLikes(ctx, userIDOf(ctx), res.Albums); err != nil {
		handleStoreError(w, err)
		return
	}
	if !s.addLikes(w, r, res.Songs) {
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (s *Server) handleCategories(w http.ResponseWriter, r *http.Request) {
	cats, err := s.store.GetCategories(r.Context())
	if err != nil {
		handleStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, cats)
}

func (s *Server) handleCategory(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := r.PathValue("id")
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
	if !s.addLikes(w, r, songs) {
		return
	}
	cat.Songs = songs
	writeJSON(w, http.StatusOK, cat)
}

func (s *Server) handleAlbums(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	albums, err := s.store.GetAlbums(ctx)
	if err != nil {
		handleStoreError(w, err)
		return
	}
	if err = s.store.ApplyAlbumLikes(ctx, userIDOf(ctx), albums); err != nil {
		handleStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, albums)
}

func (s *Server) handleAlbum(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	album, err := s.store.GetAlbum(ctx, r.PathValue("id"))
	if err != nil {
		handleStoreError(w, err)
		return
	}
	songs, err := s.store.SongsByAlbum(ctx, album.ID)
	if err != nil {
		handleStoreError(w, err)
		return
	}
	if !s.addLikes(w, r, songs) {
		return
	}
	album.Songs = songs
	liked, err := s.store.LikedSet(ctx, userIDOf(ctx), store.EntityAlbum, album.ID)
	if err != nil {
		handleStoreError(w, err)
		return
	}
	album.Liked = liked
	writeJSON(w, http.StatusOK, album)
}

func (s *Server) handleArtists(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	artists, err := s.store.GetArtists(ctx)
	if err != nil {
		handleStoreError(w, err)
		return
	}
	if err = s.store.ApplyArtistLikes(ctx, userIDOf(ctx), artists); err != nil {
		handleStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, artists)
}

// artistDetail embeds Artist, so albumCount comes from model.Artist and is the
// SQL count for the artist, not len(Albums).
type artistDetail struct {
	model.Artist
	TopSongs []model.Song  `json:"topSongs"`
	Albums   []model.Album `json:"albums"`
}

func (s *Server) handleArtist(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	artist, err := s.store.GetArtist(ctx, r.PathValue("id"))
	if err != nil {
		handleStoreError(w, err)
		return
	}
	albums, err := s.store.AlbumsByArtist(ctx, artist.ID)
	if err != nil {
		handleStoreError(w, err)
		return
	}
	top, err := s.store.TopSongsByArtist(ctx, artist.ID, 10)
	if err != nil {
		handleStoreError(w, err)
		return
	}
	if !s.addLikes(w, r, top) {
		return
	}
	liked, err := s.store.LikedSet(ctx, userIDOf(ctx), store.EntityArtist, artist.ID)
	if err != nil {
		handleStoreError(w, err)
		return
	}
	artist.Liked = liked
	writeJSON(w, http.StatusOK, artistDetail{
		Artist:   *artist,
		TopSongs: top,
		Albums:   albums,
	})
}

func (s *Server) handleSongs(w http.ResponseWriter, r *http.Request) {
	songs, err := s.store.AllSongs(r.Context())
	if err != nil {
		handleStoreError(w, err)
		return
	}
	if !s.addLikes(w, r, songs) {
		return
	}
	writeJSON(w, http.StatusOK, songs)
}

func (s *Server) handleSong(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	song, err := s.store.GetSong(ctx, r.PathValue("id"))
	if err != nil {
		handleStoreError(w, err)
		return
	}
	songs := []model.Song{*song}
	if !s.addLikes(w, r, songs) {
		return
	}
	writeJSON(w, http.StatusOK, songs[0])
}

func (s *Server) handlePlaylists(w http.ResponseWriter, r *http.Request) {
	playlists, err := s.store.GetPlaylists(r.Context(), userIDOf(r.Context()))
	if err != nil {
		handleStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, playlists)
}

func (s *Server) handlePlaylist(w http.ResponseWriter, r *http.Request) {
	pl, err := s.store.GetPlaylist(r.Context(), userIDOf(r.Context()), r.PathValue("id"))
	if err != nil {
		handleStoreError(w, err)
		return
	}
	songs := make([]model.Song, len(pl.Songs))
	for i := range pl.Songs {
		songs[i] = pl.Songs[i].Song
	}
	if !s.addLikes(w, r, songs) {
		return
	}
	for i := range pl.Songs {
		pl.Songs[i].Song = songs[i]
	}
	writeJSON(w, http.StatusOK, pl)
}

func (s *Server) handleCreatePlaylist(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name    string   `json:"name"`
		Comment string   `json:"comment"`
		SongIDs []string `json:"songIds"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	name := strings.TrimSpace(req.Name)
	if name == "" {
		writeError(w, http.StatusBadRequest, "Nome da playlist é obrigatório")
		return
	}
	pl, err := s.store.CreatePlaylist(r.Context(), userIDOf(r.Context()), name, req.Comment, req.SongIDs)
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
	if !decodeJSON(w, r, &req) {
		return
	}
	patch := &model.Playlist{Name: req.Name, Comment: req.Comment}
	if err := s.store.UpdatePlaylist(r.Context(), userIDOf(r.Context()), r.PathValue("id"), patch); err != nil {
		handleStoreError(w, err)
		return
	}
	writeNoContent(w)
}

func (s *Server) handleDeletePlaylist(w http.ResponseWriter, r *http.Request) {
	if err := s.store.DeletePlaylist(r.Context(), userIDOf(r.Context()), r.PathValue("id")); err != nil {
		handleStoreError(w, err)
		return
	}
	writeNoContent(w)
}

func (s *Server) handleAddPlaylistTracks(w http.ResponseWriter, r *http.Request) {
	var req struct {
		SongIDs []string `json:"songIds"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if err := s.store.AddPlaylistTracks(r.Context(), userIDOf(r.Context()), r.PathValue("id"), req.SongIDs); err != nil {
		handleStoreError(w, err)
		return
	}
	writeNoContent(w)
}

func (s *Server) handleRemovePlaylistTrack(w http.ResponseWriter, r *http.Request) {
	if err := s.store.RemovePlaylistTrack(r.Context(), userIDOf(r.Context()), r.PathValue("id"), r.PathValue("entryId")); err != nil {
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
	if !decodeJSON(w, r, &req) {
		return
	}
	if err := s.store.ReorderPlaylistTracks(r.Context(), userIDOf(r.Context()), r.PathValue("id"), req.From, req.To); err != nil {
		handleStoreError(w, err)
		return
	}
	writeNoContent(w)
}

func (s *Server) handleLiked(w http.ResponseWriter, r *http.Request) {
	songs, err := s.store.LikedSongs(r.Context(), userIDOf(r.Context()), 100)
	if err != nil {
		handleStoreError(w, err)
		return
	}
	if !s.addLikes(w, r, songs) {
		return
	}
	writeJSON(w, http.StatusOK, songs)
}

func (s *Server) handleLikedAlbums(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	albums, err := s.store.LikedAlbums(ctx, userIDOf(ctx), 100)
	if err != nil {
		handleStoreError(w, err)
		return
	}
	// LikedAlbums selects through user_likes, so every row is liked by
	// definition; albumCols carries no liked column.
	for i := range albums {
		albums[i].Liked = true
	}
	writeJSON(w, http.StatusOK, albums)
}

// setLike marks an entity liked or unliked. The not-found text and the existence
// probe both follow from the entity type, so callers pass only that.
func (s *Server) setLike(w http.ResponseWriter, r *http.Request, entityType string, liked bool) {
	ctx := r.Context()
	id := r.PathValue("id")
	probe := s.probeFor(entityType)
	ok, err := probe.exists(ctx, id)
	if err != nil {
		handleStoreError(w, err)
		return
	}
	if !ok {
		writeError(w, http.StatusNotFound, probe.notFound)
		return
	}
	if err = s.store.SetLike(ctx, userIDOf(ctx), entityType, id, liked); err != nil {
		handleStoreError(w, err)
		return
	}
	writeNoContent(w)
}

func (s *Server) handleLikeSong(w http.ResponseWriter, r *http.Request) {
	s.setLike(w, r, store.EntitySong, true)
}

func (s *Server) handleUnlikeSong(w http.ResponseWriter, r *http.Request) {
	s.setLike(w, r, store.EntitySong, false)
}

func (s *Server) handleLikeAlbum(w http.ResponseWriter, r *http.Request) {
	s.setLike(w, r, store.EntityAlbum, true)
}

func (s *Server) handleUnlikeAlbum(w http.ResponseWriter, r *http.Request) {
	s.setLike(w, r, store.EntityAlbum, false)
}

func (s *Server) handleHistory(w http.ResponseWriter, r *http.Request) {
	songs, err := s.store.HistorySongs(r.Context(), userIDOf(r.Context()), 50)
	if err != nil {
		handleStoreError(w, err)
		return
	}
	if !s.addLikes(w, r, songs) {
		return
	}
	writeJSON(w, http.StatusOK, songs)
}

func (s *Server) handleRegisterPlay(w http.ResponseWriter, r *http.Request) {
	if err := s.store.RegisterPlay(r.Context(), userIDOf(r.Context()), r.PathValue("id")); err != nil {
		handleStoreError(w, err)
		return
	}
	writeNoContent(w)
}

func (s *Server) handleGetQueue(w http.ResponseWriter, r *http.Request) {
	data, err := s.store.GetPlayQueue(r.Context(), userIDOf(r.Context()))
	if err != nil {
		handleStoreError(w, err)
		return
	}
	w.Header().Set("Content-Type", jsonContentType)
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

func (s *Server) handleScan(w http.ResponseWriter, r *http.Request) {
	go s.scanner.Run(context.Background())
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "scan iniciado"})
}

func (s *Server) handleAdminListUsers(w http.ResponseWriter, r *http.Request) {
	users, err := s.store.ListUsers(r.Context())
	if err != nil {
		handleStoreError(w, err)
		return
	}
	for i := range users {
		formatPhone(&users[i])
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
	if !decodeJSON(w, r, &req) {
		return
	}
	u := &model.User{Name: strings.TrimSpace(req.Name), IsAdmin: req.IsAdmin}
	var hash string
	var err error

	if req.IsAdmin {
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
		if hash, err = bcryptPassword(req.Password); err != nil {
			handleStoreError(w, err)
			return
		}
	} else {
		if strings.TrimSpace(req.Phone) == "" {
			writeError(w, http.StatusBadRequest, "Telefone é obrigatório")
			return
		}
		normalized, nerr := phone.Normalize(req.Phone)
		if nerr != nil {
			writeError(w, http.StatusBadRequest, phone.ErrInvalid.Error())
			return
		}
		u.Phone = normalized
		if hash, err = bcryptPassword(randomSecret()); err != nil {
			handleStoreError(w, err)
			return
		}
	}
	if u.Name == "" {
		u.Name = u.Username
		if u.Name == "" {
			u.Name = u.Phone
		}
	}
	u.ID = store.NewID()
	if err = s.store.CreateUser(r.Context(), u, hash, req.CategoryIDs); err != nil {
		handleStoreError(w, err)
		return
	}
	formatPhone(u)
	writeJSON(w, http.StatusCreated, u)
}

func (s *Server) handleAdminUpdateUser(w http.ResponseWriter, r *http.Request) {
	var req adminUserRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	id := r.PathValue("id")
	ctx := r.Context()
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
		SetCategories: req.CategoryIDs != nil,
		CategoryIDs:   req.CategoryIDs,
	}

	// Guard: never demote the last admin (would lock the app out).
	if current.IsAdmin && !req.IsAdmin {
		adminCount, err := s.store.CountAdmins(ctx)
		if err != nil {
			handleStoreError(w, err)
			return
		}
		if adminCount <= 1 {
			writeError(w, http.StatusBadRequest, "Não é possível remover o último administrador")
			return
		}
	}

	if req.IsAdmin {
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
		empty := ""
		patch.Phone = &empty // becoming an admin clears the phone
	} else {
		normalized, nerr := phone.Normalize(strings.TrimSpace(req.Phone))
		if nerr != nil {
			writeError(w, http.StatusBadRequest, phone.ErrInvalid.Error())
			return
		}
		patch.Phone = &normalized
		empty := ""
		patch.Username = &empty
		patch.Email = &empty
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
	if err = s.store.UpdateUser(ctx, id, patch); err != nil {
		handleStoreError(w, err)
		return
	}
	writeNoContent(w)
}

func (s *Server) handleAdminDeleteUser(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	ctx := r.Context()
	if id == userIDOf(ctx) {
		writeError(w, http.StatusBadRequest, "Não é possível excluir a própria conta")
		return
	}
	if err := s.store.DeleteUserWithPushSubscriptions(ctx, id); err != nil {
		handleStoreError(w, err)
		return
	}
	writeNoContent(w)
}

func (s *Server) handleAdminCreateCategory(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name string `json:"name"`
	}
	if !decodeJSON(w, r, &req) {
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
	songIDs, err := s.store.CategoryDetail(r.Context(), id)
	if err != nil {
		handleStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"id": id, "songIds": songIDs})
}

func (s *Server) handleAdminUpdateCategory(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Name    string   `json:"name"`
		SongIDs []string `json:"songIds"`
	}
	if !decodeJSON(w, r, &req) {
		return
	}
	if err := s.store.UpdateCategory(r.Context(), r.PathValue("id"),
		strings.TrimSpace(req.Name), req.SongIDs); err != nil {
		handleStoreError(w, err)
		return
	}
	writeNoContent(w)
}

func (s *Server) handleAdminDeleteCategory(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	id := r.PathValue("id")
	// Drop the cover too, so a recreated category cannot inherit the old one.
	if err := s.artwork.Delete(ctx, store.EntityCategory, id); err != nil {
		s.log.Warn("category cover cleanup failed", "id", id, "err", err)
	}
	if err := s.store.DeleteCategory(ctx, id); err != nil {
		handleStoreError(w, err)
		return
	}
	writeNoContent(w)
}

func (s *Server) handleAdminSongs(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	songs, err := s.store.AllSongs(ctx)
	if err != nil {
		handleStoreError(w, err)
		return
	}
	cats, err := s.store.CategorySongIDs(ctx)
	if err != nil {
		handleStoreError(w, err)
		return
	}
	categories, err := s.store.GetCategories(ctx)
	if err != nil {
		handleStoreError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"songs":        songs,
		"categoryIds":  cats,
		"categoryList": categories,
	})
}

const maxPhotoBytes = 15 << 20

// tooLargeMessage names a multipart limit, derived from it, so the wording
// cannot drift from the number it warns about.
func tooLargeMessage(limit int64) string {
	return "Arquivo muito grande (máx " + strconv.FormatInt(limit>>20, 10) + "MB) ou inválido"
}

// photoField reads the "photo" part of an already-parsed multipart form. A
// missing part yields (nil, nil); the 15 MB cap is the one every cover upload
// shares, whether the photo is required or optional.
func photoField(r *http.Request) ([]byte, error) {
	file, _, err := r.FormFile("photo")
	if err != nil {
		return nil, nil
	}
	defer file.Close()
	return io.ReadAll(io.LimitReader(file, maxPhotoBytes))
}

func readPhoto(w http.ResponseWriter, r *http.Request) ([]byte, bool) {
	r.Body = http.MaxBytesReader(w, r.Body, maxPhotoBytes)
	if err := r.ParseMultipartForm(maxPhotoBytes); err != nil {
		writeError(w, http.StatusBadRequest, tooLargeMessage(maxPhotoBytes))
		return nil, false
	}
	data, err := photoField(r)
	if err != nil {
		writeError(w, http.StatusBadRequest, "Falha ao ler arquivo")
		return nil, false
	}
	if data == nil {
		writeError(w, http.StatusBadRequest, "Campo 'photo' obrigatório")
		return nil, false
	}
	return data, true
}

// entityProbe is the single table of per-entity-type facts the like and artwork
// routes need: how to tell the row exists, and the not-found wording. An
// unknown type is a programming error, not a runtime case.
type entityProbe struct {
	exists   func(context.Context, string) (bool, error)
	notFound string
}

func (s *Server) probeFor(entityType string) entityProbe {
	switch entityType {
	case store.EntitySong:
		return entityProbe{exists: s.store.SongExists, notFound: "Música não encontrada"}
	case store.EntityAlbum:
		return entityProbe{exists: s.store.AlbumExists, notFound: "Álbum não encontrado"}
	case store.EntityCategory:
		return entityProbe{exists: func(ctx context.Context, id string) (bool, error) {
			_, err := s.store.GetCategory(ctx, id)
			if errors.Is(err, store.ErrNotFound) {
				return false, nil
			}
			return err == nil, err
		}, notFound: "Categoria não encontrada"}
	default:
		panic("probeFor: unsupported entity type " + entityType)
	}
}

func (s *Server) uploadPhoto(entityType string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		probe := s.probeFor(entityType)
		ok, err := probe.exists(r.Context(), id)
		if err != nil {
			handleStoreError(w, err)
			return
		}
		if !ok {
			writeError(w, http.StatusNotFound, probe.notFound)
			return
		}
		data, ok := readPhoto(w, r)
		if !ok {
			return
		}
		if err = s.artwork.Upload(r.Context(), entityType, id, data); err != nil {
			writeError(w, http.StatusBadRequest, err.Error())
			return
		}
		writeNoContent(w)
	}
}

func (s *Server) deletePhoto(entityType string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := s.artwork.Delete(r.Context(), entityType, r.PathValue("id")); err != nil {
			handleStoreError(w, err)
			return
		}
		writeNoContent(w)
	}
}

const maxUploadBytes = 512 << 20

func (s *Server) handleAdminUploadSong(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxUploadBytes)
	if err := r.ParseMultipartForm(maxUploadBytes); err != nil {
		writeError(w, http.StatusBadRequest, tooLargeMessage(maxUploadBytes))
		return
	}
	file, header, err := r.FormFile("song")
	if err != nil {
		writeError(w, http.StatusBadRequest, "Campo 'song' obrigatório (arquivo de áudio)")
		return
	}
	defer file.Close()

	ext := strings.ToLower(filepath.Ext(header.Filename))
	contentType, ok := stream.MIMEForExt(ext)
	if !ok {
		writeError(w, http.StatusBadRequest, "Formato de áudio não suportado: "+ext)
		return
	}

	tmp, err := os.CreateTemp("", "pm-upload-*"+ext)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "Falha ao processar arquivo")
		return
	}
	defer os.Remove(tmp.Name())
	size, err := io.Copy(tmp, file)
	if err != nil {
		tmp.Close()
		writeError(w, http.StatusBadRequest, "Falha ao ler arquivo")
		return
	}
	if size == 0 {
		tmp.Close()
		writeError(w, http.StatusBadRequest, "Arquivo vazio")
		return
	}
	if err := tmp.Close(); err != nil {
		writeError(w, http.StatusInternalServerError, "Falha ao processar arquivo")
		return
	}

	// Rejected before it reaches the bucket: a duration-0 song must never enter
	// the catalog.
	if _, err := metadata.ReadAudio(tmp.Name(), size, s.cfg.FfmpegPath); err != nil {
		writeError(w, http.StatusBadRequest, metadata.ErrInvalidAudio.Error())
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
		s.storage.WaitForObject(r.Context(), key)
		song, err = s.scanner.IndexFile(r.Context(), key, header.Filename)
	}
	if err != nil {
		s.log.Error("upload index", "key", key, "err", err)
		if rerr := s.storage.Remove(r.Context(), key); rerr != nil {
			s.log.Warn("upload cleanup", "key", key, "err", rerr)
		}
		if errors.Is(err, metadata.ErrInvalidAudio) {
			writeError(w, http.StatusBadRequest, metadata.ErrInvalidAudio.Error())
			return
		}
		writeError(w, http.StatusInternalServerError, "Falha ao indexar a música")
		return
	}

	title := strings.TrimSpace(r.FormValue("title"))
	artist := strings.TrimSpace(r.FormValue("artist"))
	if title != "" || artist != "" {
		if err := s.store.UpdateSongMeta(r.Context(), song.ID, title, artist); err != nil {
			s.log.Warn("upload meta update", "err", err)
		}
	}

	// An unknown category is a client mistake, not a warning: the upload would
	// otherwise succeed silently without the file landing where asked.
	if categoryID := strings.TrimSpace(r.FormValue("categoryId")); categoryID != "" {
		category := s.probeFor(store.EntityCategory)
		ok, err := category.exists(r.Context(), categoryID)
		if err != nil {
			handleStoreError(w, err)
			return
		}
		if !ok {
			writeError(w, http.StatusBadRequest, category.notFound)
			return
		}
		if err := s.store.AddSongToCategory(r.Context(), categoryID, song.ID); err != nil {
			s.log.Warn("upload category assign", "err", err)
		}
	}

	if data, err := photoField(r); err != nil {
		s.log.Warn("upload photo read", "err", err)
	} else if len(data) > 0 {
		if err := s.artwork.Upload(r.Context(), store.EntitySong, song.ID, data); err != nil {
			s.log.Warn("upload photo", "err", err)
		}
	}

	// The response must carry the metadata the caller just set, so re-read it.
	// A failed re-read is not worth failing the upload: the song exists.
	if fresh, err := s.store.GetSong(r.Context(), song.ID); err == nil {
		song = fresh
	} else {
		s.log.Warn("upload reload", "id", song.ID, "err", err)
	}

	writeJSON(w, http.StatusCreated, song)
}

func (s *Server) addLikes(w http.ResponseWriter, r *http.Request, songs []model.Song) bool {
	if err := s.store.ApplySongLikes(r.Context(), userIDOf(r.Context()), songs); err != nil {
		handleStoreError(w, err)
		return false
	}
	return true
}
