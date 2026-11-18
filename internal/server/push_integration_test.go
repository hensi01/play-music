package server

import (
	"context"
	"io"
	"log/slog"
	"play-music/internal/config"
	"play-music/internal/db"
	"play-music/internal/store"
	"sync"
	"testing"
)

func TestCampaignClaimConcurrentPostgres(t *testing.T) {
	databaseURL := testDisposableDB(t)
	ctx := context.Background()
	pool, err := db.Connect(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if err = db.Migrate(ctx, pool, slog.New(slog.NewTextHandler(io.Discard, nil))); err != nil {
		t.Fatal(err)
	}
	// Prevent an integration test from dispatching any pre-existing campaign.
	var existing int
	if err = pool.QueryRow(ctx, "SELECT count(*) FROM push_campaigns WHERE status IN ('pending','sending')").Scan(&existing); err != nil || existing != 0 {
		t.Fatal("test database contains pending campaigns")
	}
	id := store.NewID()
	_, err = pool.Exec(ctx, `INSERT INTO push_campaigns(id,title,body,url,scheduled_at,status) VALUES($1,'Test','Test','/',now(),'pending')`, id)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _, _ = pool.Exec(context.Background(), "DELETE FROM push_campaigns WHERE id=$1", id) }()
	s := &Server{store: store.New(pool), cfg: &config.Config{PushProvider: "relay", RelayAppID: "test", RelayAppSecret: "test", RelayVapidPublicKey: "test"}}
	// A fresh test DB has no recipients, so no external push request is made.
	var count int
	if err = pool.QueryRow(ctx, "SELECT count(*) FROM push_subscriptions WHERE user_id IS NOT NULL AND active").Scan(&count); err != nil || count != 0 {
		t.Fatal("test database contains active subscriptions")
	}
	var wg sync.WaitGroup
	for i := 0; i < 2; i++ {
		wg.Add(1)
		go func() { defer wg.Done(); s.runPushCampaign(ctx) }()
	}
	wg.Wait()
	var status string
	var selected int
	if err = pool.QueryRow(ctx, "SELECT status,selected FROM push_campaigns WHERE id=$1", id).Scan(&status, &selected); err != nil || status != "completed" || selected != 0 {
		t.Fatalf("status=%s selected=%d err=%v", status, selected, err)
	}
}
