package stats

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/yetone/magpie/internal/catalog"
	"github.com/yetone/magpie/internal/provider"
	"github.com/yetone/magpie/internal/settings"
)

func TestSend(t *testing.T) {
	cfg := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", cfg)
	t.Setenv("XDG_CACHE_HOME", t.TempDir())
	t.Setenv("DO_NOT_TRACK", "")
	t.Setenv("MAGPIE_NO_STATS", "")
	var got []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		got = append(got, r.Method+" "+r.URL.String()+" "+string(b))
	}))
	defer srv.Close()
	t.Setenv("MAGPIE_STATS_HOST", srv.URL)
	day := time.Date(2026, 9, 28, 9, 0, 0, 0, time.UTC)
	for _, at := range []time.Time{day, day.Add(3 * time.Hour), day.Add(24 * time.Hour)} {
		if err := Send(context.Background(), "0.1.300", "app", at); err != nil {
			t.Fatal(err)
		}
	}
	if err := settings.Save(settings.Settings{NoStats: true}); err != nil {
		t.Fatal(err)
	}
	if err := Send(context.Background(), "0.1.300", "app", day.Add(48*time.Hour)); err != nil {
		t.Fatal(err)
	}
	t.Setenv("MAGPIE_NO_STATS", "1")
	if err := settings.Save(settings.Settings{}); err != nil {
		t.Fatal(err)
	}
	if err := Send(context.Background(), "0.1.300", "app", day.Add(72*time.Hour)); err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("handler ran: %v", got)
	}
	err := filepath.Walk(cfg, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.Name() == "install-id" || info.Name() == "stats-sent" {
			t.Errorf("%s appeared under %s", info.Name(), cfg)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// A provider is named by what magpie built in, never by what the user
// typed: a preset's id, a subscription's agent, a known plugin's id;
// anything else is "custom" or "plugin".
func TestProviderLabel(t *testing.T) {
	for _, c := range []struct {
		p    provider.Provider
		want string
	}{
		{provider.Provider{ID: "my-deepseek", Name: "My DeepSeek", Preset: "deepseek", Chat: "https://api.deepseek.com/v1"}, "deepseek"},
		{provider.Provider{ID: "secret-relay", Name: "Secret Relay", Chat: "https://relay.example/v1"}, "custom"},
		{provider.Provider{ID: "odd", Name: "Odd", Preset: "no-such-preset"}, "custom"},
		{provider.Provider{ID: "codex", Account: &provider.Account{Agent: "codex", User: "someone@example.com"}}, "codex"},
	} {
		if got := providerLabel(c.p); got != c.want {
			t.Errorf("%s: %q, want %q", c.p.ID, got, c.want)
		}
	}
}

// Nothing the user named goes as an agent: an omp profile, a WSL distro.
func TestAgentLabel(t *testing.T) {
	for id, want := range map[string]string{
		"claude":                   "claude",
		"claude@wsl:Ubuntu":        "claude",
		"omp#acme-secret":          "omp",
		"omp#acme-secret@wsl:Work": "omp",
	} {
		if got := agentLabel(id); got != want {
			t.Errorf("%s: %q, want %q", id, got, want)
		}
	}
}

// A model on a key's provider goes by its id only when models.dev knows
// it: one the user named (an Ollama model, a fine-tune naming their
// organisation, or a known one spelt their way) is "other".
func TestModelLabelKeepsTheUsersOwnModelIDs(t *testing.T) {
	cache := map[string]any{"deepseek": map[string]any{"id": "deepseek", "models": map[string]any{
		"deepseek-v4": map[string]any{"id": "deepseek-v4", "limit": map[string]any{"context": 128000}},
	}}}
	b, _ := json.Marshal(cache)
	if err := os.MkdirAll(filepath.Dir(catalog.CachePath()), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(catalog.CachePath(), b, 0o600); err != nil {
		t.Fatal(err)
	}
	catalog.Reset()
	t.Cleanup(func() { os.Remove(catalog.CachePath()); catalog.Reset() })
	for _, p := range []provider.Provider{
		{ID: "my-deepseek", Name: "DS", Preset: "deepseek", Chat: "https://api.deepseek.com/v1", Key: "sk-x", Models: []string{"deepseek-v4", "DeepSeek_V4-260101"}},
		{ID: "my-openai", Name: "OA", Preset: "openai", Chat: "https://api.openai.com/v1", Key: "sk-y", Models: []string{"ft:gpt-4o:acme-corp::abc123"}},
		{ID: "my-ollama", Name: "Local", Preset: "ollama", Chat: "http://127.0.0.1:11434/v1", Models: []string{"acme-internal-llm"}},
	} {
		if err := provider.Save(p); err != nil {
			t.Fatal(err)
		}
	}
	for ref, want := range map[string]string{
		"my-deepseek/deepseek-v4": "deepseek/deepseek-v4",
		// known only under another spelling (catalog.SameModel): the
		// user's own spelling isn't sent
		"my-deepseek/DeepSeek_V4-260101":        "deepseek/other",
		"my-openai/ft:gpt-4o:acme-corp::abc123": "openai/other",
		"my-ollama/acme-internal-llm":           "ollama/other",
	} {
		if got := modelLabel(ref); got != want {
			t.Errorf("%s: %q, want %q", ref, got, want)
		}
	}
}

func TestSendPartnerCounts(t *testing.T) {
	home := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(home, ".config"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(home, ".cache"))
	t.Setenv("DO_NOT_TRACK", "")
	t.Setenv("MAGPIE_NO_STATS", "")
	counts := []byte(`{"days":{"2026-09-27":{"acme":{"shown":12,"opened":2}},"2026-09-28":{"acme":{"shown":3}}}}`)
	f := filepath.Join(home, ".cache", "magpie", "partner-counts.json")
	if err := os.MkdirAll(filepath.Dir(f), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(f, counts, 0o644); err != nil {
		t.Fatal(err)
	}
	var got []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		got = append(got, r.Method+" "+r.URL.String()+" "+string(b))
	}))
	defer srv.Close()
	t.Setenv("MAGPIE_STATS_HOST", srv.URL)
	if err := Send(context.Background(), "0.1.300", "app", time.Date(2026, 9, 28, 9, 0, 0, 0, time.UTC)); err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("handler ran: %v", got)
	}
	after, err := os.ReadFile(f)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(after, counts) {
		t.Fatalf("partner-counts.json changed from %q to %q", counts, after)
	}
}
