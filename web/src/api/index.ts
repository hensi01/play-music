import { api, getToken } from './client'
import type {
  Album,
  AlbumDetail,
  Artist,
  ArtistDetail,
  Home,
  LyricsResponse,
  Playlist,
  PlaylistDetail,
  Queue,
  SearchResults,
  Settings,
  Song,
  UserProfile,
} from './types'

export const endpoints = {
  me: () => api.get<UserProfile>('/api/me'),
  settings: () => api.get<Settings>('/api/settings'),
  home: () => api.get<Home>('/api/home'),
  search: (q: string) => api.get<SearchResults>(`/api/search?q=${encodeURIComponent(q)}`),

  albums: (params = '') => api.get<Album[]>(`/api/albums${params}`),
  album: (id: string) => api.get<AlbumDetail>(`/api/albums/${id}`),
  artists: () => api.get<Artist[]>('/api/artists'),
  artist: (id: string) => api.get<ArtistDetail>(`/api/artists/${id}`),
  song: (id: string) => api.get<Song>(`/api/songs/${id}`),

  playlists: () => api.get<Playlist[]>('/api/playlists'),
  playlist: (id: string) => api.get<PlaylistDetail>(`/api/playlists/${id}`),
  createPlaylist: (name: string, songIds: string[]) =>
    api.post<{ id: string }>('/api/playlists', { name, songIds }),
  updatePlaylist: (
    id: string,
    body: { name?: string; comment?: string; public?: boolean; addSongIds?: string[]; removeIndexes?: number[] },
  ) => api.put<{ id: string }>(`/api/playlists/${id}`, body),
  deletePlaylist: (id: string) => api.del<void>(`/api/playlists/${id}`),
  addPlaylistTracks: (id: string, songIds: string[]) =>
    api.post<{ id: string }>(`/api/playlists/${id}/tracks`, { songIds }),
  removePlaylistTrack: (id: string, entryId: string) => api.del<void>(`/api/playlists/${id}/tracks/${entryId}`),
  reorderPlaylistTracks: (id: string, from: number, to: number) =>
    api.put<{ id: string }>(`/api/playlists/${id}/tracks`, { from, to }),

  liked: () => api.get<Song[]>('/api/me/liked'),
  like: (id: string) => api.put<void>(`/api/me/liked/${id}`),
  unlike: (id: string) => api.del<void>(`/api/me/liked/${id}`),
  history: () => api.get<Song[]>('/api/me/history'),
  registerPlay: (id: string) => api.post<void>(`/api/me/history/${id}`),

  queue: () => api.get<Queue>('/api/queue'),
  saveQueue: (q: { current: number; position: number; songIds: string[] }) => api.put<void>('/api/queue', q),

  lyrics: (id: string) => api.get<LyricsResponse>(`/api/lyrics/${id}`),
}

export function artworkUrl(id: string, size = 300): string {
  // <img> tags cannot send the Authorization header, so the JWT travels as a
  // query parameter (the server accepts ?jwt= via jwtauth.TokenFromQuery).
  const q = new URLSearchParams({ size: String(size) })
  const token = getToken()
  if (token) q.set('jwt', token)
  return `/api/artwork/${id}?${q.toString()}`
}

// Formats browsers can usually play natively; anything else is transcoded to mp3.
const nativeFormats = new Set(['mp3', 'm4a', 'aac', 'ogg', 'opus', 'wav', 'flac'])

export function streamUrl(song: Song, fallback = false): string {
  // <audio> cannot send the Authorization header, so the JWT travels as a query
  // parameter (the server accepts ?jwt= via jwtauth.TokenFromQuery).
  const q = new URLSearchParams()
  if (fallback || !nativeFormats.has(song.format.toLowerCase())) q.set('format', 'mp3')
  const token = getToken()
  if (token) q.set('jwt', token)
  const qs = q.toString()
  return `/api/stream/${song.id}${qs ? `?${qs}` : ''}`
}
