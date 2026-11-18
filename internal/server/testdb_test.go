package server

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// appDBURL resolves DATABASE_URL from the environment, falling back to the
// repo .env. Empty means the package-level fixtures cannot run.
func appDBURL(t *testing.T) string {
	t.Helper()
	if u := os.Getenv("DATABASE_URL"); u != "" {
		return u
	}
	return dotEnv(t)["DATABASE_URL"]
}

// testDisposableDB skips unless TEST_DATABASE_URL names a throwaway database.
// The tests using it leave rows (and storage objects) behind on failure, so it
// must never be the database the app runs on.
func testDisposableDB(t *testing.T) string {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("TEST_DATABASE_URL not configured for disposable database")
	}
	if url == appDBURL(t) {
		t.Fatal("TEST_DATABASE_URL must not be the database the app runs on")
	}
	return url
}

// dotEnv parses the repo .env (repo root, reached from the package dir).
func dotEnv(t *testing.T) map[string]string {
	t.Helper()
	out := map[string]string{}
	root, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Skipf("resolve repo root: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(root, ".env"))
	if err != nil {
		return out
	}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if k, v, ok := strings.Cut(line, "="); ok {
			out[strings.TrimSpace(k)] = strings.Trim(strings.TrimSpace(v), `"'`)
		}
	}
	return out
}
