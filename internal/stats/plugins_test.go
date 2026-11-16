package stats

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// This build tells usemagpie.ai nothing of the plugins here: the day's
// event sends no report, a like is refused, and no install id is made.
func TestSendPlugins(t *testing.T) {
	cfg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", cfg)
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	t.Setenv("DO_NOT_TRACK", "")
	t.Setenv("MAGPIE_NO_STATS", "")
	was := publicPlugins
	publicPlugins = func(context.Context) []string { return []string{"opencode-x"} }
	defer func() { publicPlugins = was }()
	var got []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		got = append(got, r.Method+" "+r.URL.String()+" "+string(b))
	}))
	defer srv.Close()
	t.Setenv("MAGPIE_STATS_HOST", srv.URL)
	t.Setenv("MAGPIE_SITE_HOST", srv.URL)

	if err := Send(context.Background(), "0.1.300", "app", time.Date(2026, 11, 11, 9, 0, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}
	if _, err := Like(context.Background(), "opencode-x", true); err == nil {
		t.Fatal("like went through")
	}
	if len(got) != 0 {
		t.Fatalf("handler ran: %v", got)
	}
	if _, err := os.Stat(filepath.Join(cfg, "magpie", "install-id")); !os.IsNotExist(err) {
		t.Fatalf("install-id: %v", err)
	}
}
