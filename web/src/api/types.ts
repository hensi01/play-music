export interface Song {
  id: string
  title: string
  albumId: string
  album: string
  artistId: string
  artist: string
  albumArtist: string
  duration: number
  trackNumber: number
  discNumber: number
  year: number
  genre?: string
  format: string
  bitRate: number
  playCount: number
  liked: boolean
  lastPlayedAt?: string
}

export interface Album {
  id: string
  name: string
  artistId: string
  artist: string
  year: number
  genre?: string
  songCount: number
  duration: number
  playCount: number
  liked: boolean
  createdAt?: string
}

export interface Artist {
  id: string
  name: string
  albumCount: number
  songCount: number
  liked: boolean
}

export interface Playlist {
  id: string
  name: string
  comment?: string
  songCount: number
  duration: number
  owner: string
  public: boolean
  updatedAt: string
}

export interface PlaylistSong {
  entryId: string
  song: Song
}

export interface Genre {
  name: string
  songCount: number
  albumCount: number
}

export interface UserProfile {
  id: string
  name: string
  username: string
  email?: string
  isAdmin: boolean
}

export interface Settings {
  appName: string
  version: string
  libraryName: string
  musicFolder: string
}

export interface Section {
  id: string
  title: string
  albums?: Album[]
  songs?: Song[]
  genres?: Genre[]
}

export interface Home {
  sections: Section[]
  genres: Genre[]
}

export interface SearchResults {
  songs: Song[]
  albums: Album[]
  artists: Artist[]
  playlists: Playlist[]
}

export interface AlbumDetail extends Album {
  songs: Song[]
}

export interface ArtistDetail extends Artist {
  albums: Album[]
  topSongs: Song[]
}

export interface PlaylistDetail extends Playlist {
  songs: PlaylistSong[]
}

export interface LyricsResponse {
  synced: boolean
  source?: string
  lines: { start?: number; end?: number; text: string }[]
}

export interface Queue {
  current: number
  position: number
  songs: Song[]
}
