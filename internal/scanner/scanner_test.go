package scanner

import (
	"context"
	"errors"
	"github.com/minio/minio-go/v7"
	"testing"
)

func TestCollectInventoryRejectsPartialListing(t *testing.T) {
	objects := make(chan minio.ObjectInfo, 2)
	objects <- minio.ObjectInfo{Key: "songs/a.mp3", Size: 42}
	objects <- minio.ObjectInfo{Err: errors.New("storage unavailable")}
	close(objects)
	songs, covers, lists, seen, err := collectInventory(context.Background(), objects)
	if err == nil || songs != nil || covers != nil || lists != nil || seen != nil {
		t.Fatal("partial inventory must never be used")
	}
}
func TestCollectInventoryCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	objects := make(chan minio.ObjectInfo)
	close(objects)
	_, _, _, _, err := collectInventory(ctx, objects)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("got %v", err)
	}
}
func TestCollectInventoryClassifiesAudioAndIgnoresHLS(t *testing.T) {
	objects := make(chan minio.ObjectInfo, 4)
	objects <- minio.ObjectInfo{Key: "Album/song.MP3", Size: 42}
	objects <- minio.ObjectInfo{Key: "Album/cover.jpg", Size: 12}
	objects <- minio.ObjectInfo{Key: "lists/music.m3u", Size: 5}
	objects <- minio.ObjectInfo{Key: "videos/playlist.m3u8", Size: 5}
	close(objects)
	songs, covers, lists, seen, err := collectInventory(context.Background(), objects)
	if err != nil {
		t.Fatal(err)
	}
	if len(songs) != 1 || songs[0].key != "Album/song.MP3" || covers["Album/"] != "Album/cover.jpg" || len(lists) != 1 || lists[0] != "lists/music.m3u" || len(seen) != 4 {
		t.Fatalf("inventory classification was incorrect")
	}
}
