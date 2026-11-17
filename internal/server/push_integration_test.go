package server

import (
	"context"
	"io"
	"log/slog"
	"net/http/httptest"
	"os"
	"play-music/internal/config"
	"play-music/internal/db"
	"play-music/internal/store"
	"strings"
	"sync"
	"testing"
)

// TEST_DATABASE_URL must name a disposable, dedicated PostgreSQL database.
func TestCampaignClaimConcurrentPostgres(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL not configured for disposable database")
	}
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
func TestSharedPushSubscriptionPostgres(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("dedicated PostgreSQL required")
	}
	ctx := context.Background()
	pool, err := db.Connect(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if err = db.Migrate(ctx, pool, slog.New(slog.NewTextHandler(io.Discard, nil))); err != nil {
		t.Fatal(err)
	}
	a, b := store.NewID(), store.NewID()
	endpoint := "https://fcm.googleapis.com/fcm/send/" + store.NewID()
	_, err = pool.Exec(ctx, "INSERT INTO users(id,password_hash) VALUES($1,''),($2,'')", a, b)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Exec(context.Background(), "DELETE FROM users WHERE id IN ($1,$2)", a, b)
	body := `{"endpoint":"` + endpoint + `","expirationTime":null,"keys":{"p256dh":"key","auth":"secret"}}`
	_, err = pool.Exec(ctx, "INSERT INTO push_subscriptions(endpoint,user_id,relay_id,management_token,subscription_data,active) VALUES($1,$2,$3,'token',$4,true)", endpoint, a, "relay-"+store.NewID(), []byte(body))
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Exec(context.Background(), "DELETE FROM push_subscriptions WHERE endpoint=$1", endpoint)
	_, err = pool.Exec(ctx, "INSERT INTO push_subscription_users(endpoint,user_id) VALUES($1,$2)", endpoint, a)
	if err != nil {
		t.Fatal(err)
	}
	s := &Server{store: store.New(pool), cfg: &config.Config{PushProvider: "relay", RelayAppID: "app", RelayAppSecret: "secret", RelayVapidPublicKey: "key"}}
	request := func(user, method, payload string) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, "/", strings.NewReader(payload))
		r = r.WithContext(context.WithValue(r.Context(), ctxUserID, user))
		w := httptest.NewRecorder()
		if method == "POST" {
			s.handlePushSubscribe(w, r)
		} else {
			s.handlePushUnsubscribe(w, r)
		}
		return w
	}
	// A pending registration cancelled by its owner cannot later commit using the old operation id.
	pending := endpoint + "-pending"
	_, err = pool.Exec(ctx, "INSERT INTO push_subscriptions(endpoint,user_id,relay_id,management_token,subscription_data,active,operation_id) VALUES($1,$2,$3,'',$4,false,'old')", pending, b, "pending-"+store.NewID(), []byte(body))
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Exec(context.Background(), "DELETE FROM push_subscriptions WHERE endpoint=$1", pending)
	if w := request(b, "DELETE", `{"endpoint":"`+pending+`"}`); w.Code != 204 {
		t.Fatalf("pending delete: %d", w.Code)
	}
	var operation string
	var owner *string
	if err = pool.QueryRow(ctx, "SELECT operation_id,user_id FROM push_subscriptions WHERE endpoint=$1", pending).Scan(&operation, &owner); err != nil || operation == "old" || owner != nil {
		t.Fatalf("pending cancellation operation=%q owner=%v err=%v", operation, owner, err)
	}
	if w := request(b, "POST", body); w.Code != 200 {
		t.Fatalf("second account: %d %s", w.Code, w.Body.String())
	}
	// Removing one account must not revoke the device for the other account.
	if err = s.removeUserPushSubscriptions(ctx, a); err != nil {
		t.Fatal(err)
	}
	var remaining bool
	if err = pool.QueryRow(ctx, "SELECT active FROM push_subscriptions WHERE endpoint=$1", endpoint).Scan(&remaining); err != nil || !remaining {
		t.Fatalf("other account lost push: active=%v err=%v", remaining, err)
	}
	if w := request(a, "DELETE", `{"endpoint":"`+endpoint+`"}`); w.Code != 204 {
		t.Fatalf("idempotent deletion: %d", w.Code)
	}
	if w := request(a, "DELETE", `{"endpoint":"`+endpoint+`"}`); w.Code != 204 {
		t.Fatalf("delete: %d %s", w.Code, w.Body.String())
	}
	var active bool
	var count int
	err = pool.QueryRow(ctx, "SELECT p.active,(SELECT count(*) FROM push_subscription_users WHERE endpoint=p.endpoint) FROM push_subscriptions p WHERE endpoint=$1", endpoint).Scan(&active, &count)
	if err != nil || !active || count != 1 {
		t.Fatalf("remaining active=%v count=%d err=%v", active, count, err)
	}
	if w := request(b, "DELETE", `{"endpoint":"`+endpoint+`"}`); w.Code != 204 {
		t.Fatalf("last delete: %d", w.Code)
	}
	err = pool.QueryRow(ctx, "SELECT active FROM push_subscriptions WHERE endpoint=$1", endpoint).Scan(&active)
	if err != nil || active {
		t.Fatalf("last active=%v err=%v", active, err)
	}
}

func TestCampaignRepeatPostgres(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("dedicated PostgreSQL required")
	}
	ctx := context.Background()
	pool, err := db.Connect(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if err = db.Migrate(ctx, pool, slog.New(slog.NewTextHandler(io.Discard, nil))); err != nil {
		t.Fatal(err)
	}
	id := store.NewID()
	_, err = pool.Exec(ctx, `INSERT INTO push_campaigns(id,title,body,url,scheduled_at,status,repeat_hours) VALUES($1,'Test','Test','/',now(),'pending',2)`, id)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Exec(ctx, `DELETE FROM push_campaigns WHERE id=$1`, id)
	s := &Server{store: store.New(pool), cfg: &config.Config{PushProvider: "relay", RelayAppID: "test", RelayAppSecret: "test", RelayVapidPublicKey: "test"}}
	s.runPushCampaign(ctx)
	var published int
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM notification_inbox WHERE title='Test' AND body='Test'`).Scan(&published); err != nil || published != 1 {
		t.Fatalf("published=%d err=%v", published, err)
	}
	inbox := httptest.NewRecorder()
	s.handleNotificationInbox(inbox, httptest.NewRequest("GET", "/api/notifications", nil))
	if inbox.Code != 200 || !strings.Contains(inbox.Body.String(), `"title":"Test"`) {
		t.Fatalf("inbox=%d %s", inbox.Code, inbox.Body.String())
	}
	var status string
	var hours int
	var future bool
	err = pool.QueryRow(ctx, `SELECT status,repeat_hours,scheduled_at > now() + interval '119 minutes' FROM push_campaigns WHERE id=$1`, id).Scan(&status, &hours, &future)
	if err != nil || status != "pending" || hours != 2 || !future {
		t.Fatalf("status=%s hours=%d future=%v err=%v", status, hours, future, err)
	}
	_, err = pool.Exec(ctx, `UPDATE push_campaigns SET status='sending' WHERE id=$1`, id)
	if err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest("DELETE", "/", nil)
	req.SetPathValue("id", id)
	w := httptest.NewRecorder()
	s.handleCancelCampaign(w, req)
	if w.Code != 204 {
		t.Fatalf("cancel: %d", w.Code)
	}
	s.finishCampaign(id, "completed", 0, 0, "")
	err = pool.QueryRow(ctx, `SELECT status,repeat_hours FROM push_campaigns WHERE id=$1`, id).Scan(&status, &hours)
	if err != nil || status != "completed" || hours != 0 {
		t.Fatalf("status=%s hours=%d err=%v", status, hours, err)
	}
	_, err = pool.Exec(ctx, `UPDATE push_campaigns SET status='sending',repeat_hours=2 WHERE id=$1`, id)
	if err != nil {
		t.Fatal(err)
	}
	s.finishCampaign(id, "unknown", 0, 0, "uncertain")
	err = pool.QueryRow(ctx, `SELECT status,repeat_hours FROM push_campaigns WHERE id=$1`, id).Scan(&status, &hours)
	if err != nil || status != "unknown" || hours != 0 {
		t.Fatalf("status=%s hours=%d err=%v", status, hours, err)
	}
}
