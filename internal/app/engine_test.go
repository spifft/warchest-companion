package app

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/spifft/warchest-companion/internal/api"
	"github.com/spifft/warchest-companion/internal/config"
)

// a stand-in for warchestmod.com: pairing codes and the relay
type fakeSite struct {
	mu       sync.Mutex
	approved bool
	status   int // what /relay answers (0 = 200)
	got      [][]map[string]string
	auth     []string
}

func (f *fakeSite) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	f.mu.Lock()
	defer f.mu.Unlock()
	switch r.URL.Path {
	case "/rest/v1/rpc/companion_start":
		json.NewEncoder(w).Encode(map[string]any{"code": "ABCD-EFGH", "expires": "2030-01-01T00:00:00Z"})
	case "/rest/v1/rpc/companion_claim":
		if !f.approved {
			json.NewEncoder(w).Encode(map[string]any{"status": "waiting"})
			return
		}
		json.NewEncoder(w).Encode(map[string]any{"status": "ok", "token": "wcd_" + strings.Repeat("ab", 32), "account": "LANrat", "char": map[string]any{"n": "Brannoc", "v": true}})
	case "/relay":
		f.auth = append(f.auth, r.Header.Get("Authorization"))
		var body struct {
			Lines []map[string]string `json:"lines"`
		}
		json.NewDecoder(r.Body).Decode(&body)
		if f.status != 0 {
			w.WriteHeader(f.status)
			json.NewEncoder(w).Encode(map[string]any{"error": map[int]string{401: "This computer was disconnected.", 409: "Link your character with /wc link first."}[f.status], "code": "unverified"})
			return
		}
		f.got = append(f.got, body.Lines)
		json.NewEncoder(w).Encode(map[string]any{"ok": true, "added": len(body.Lines), "room": "us-normal-alliance", "char": "Brannoc"})
	default:
		http.NotFound(w, r)
	}
}

func TestEngine(t *testing.T) {
	t.Setenv("XDG_CONFIG_HOME", t.TempDir())
	t.Setenv("HOME", t.TempDir())
	site := &fakeSite{}
	srv := httptest.NewServer(site)
	defer srv.Close()
	wow := t.TempDir()
	os.MkdirAll(filepath.Join(wow, "Logs"), 0o755)
	os.MkdirAll(filepath.Join(wow, "WTF"), 0o755)
	logp := filepath.Join(wow, "Logs", "WoWChatLog.txt")
	os.WriteFile(logp, []byte("10/18 10:00:00.000  [2. Trade] Old: before the app started\n"), 0o644)

	c := api.New("test")
	c.SupabaseURL, c.LiveURL, c.SiteURL = srv.URL, srv.URL, "https://warchestmod.com"
	cfg := config.Load()
	cfg.WoWDir = wow
	var opened string
	var notes []string
	e := &Engine{Client: c, Cfg: cfg, Open: func(u string) { opened = u }, Notify: func(_, m string) { notes = append(notes, m) }}

	// pairing: a code, the approval page, then the key once approved
	if err := e.Pair("Test PC"); err != nil {
		t.Fatal(err)
	}
	if e.Status().State != Pairing || e.Status().Code != "ABCD-EFGH" || opened != "https://warchestmod.com/portal.html#/companion/ABCD-EFGH" {
		t.Fatalf("pairing: %+v %q", e.Status(), opened)
	}
	e.Step()
	if e.Status().State != Pairing {
		t.Fatal("should still be waiting for approval")
	}
	site.approved = true
	e.Step()
	if e.Cfg.Token() == "" || e.Status().Account != "LANrat" || config.Load().Token() == "" {
		t.Fatalf("the key should be saved after approval: %+v", e.Status())
	}

	// relaying: the first round starts at the end of the log; then only Trade lines go out
	e.Step()
	f, _ := os.OpenFile(logp, os.O_APPEND|os.O_WRONLY, 0o644)
	f.WriteString("10/18 10:00:01.000  [2. Trade - City] Thoria-Forever: WTS [Thorium Ore] 5g\n" +
		"10/18 10:00:02.000  [Brannoc] whispers: my secret\n" +
		"10/18 10:00:03.000  [Guild] Brannoc: guild stuff\n" +
		"10/18 10:00:04.000  [2. Trade - City] Kaelith: LF enchanter\n")
	f.Close()
	e.Step()
	var sent []map[string]string
	for _, b := range site.got {
		sent = append(sent, b...)
	}
	if len(sent) != 2 || sent[0]["n"] != "Thoria" || sent[0]["t"] != "WTS [Thorium Ore] 5g" || sent[1]["n"] != "Kaelith" {
		t.Fatalf("only the two Trade lines should be sent, got %v", sent)
	}
	for _, a := range site.auth {
		if a != "Bearer wcd_"+strings.Repeat("ab", 32) {
			t.Fatalf("relay must send the device key, got %q", a)
		}
	}
	if s := e.Status(); s.State != Relaying || s.Room != "us-normal-alliance" || s.Sent != 2 {
		t.Fatalf("status after relaying: %+v", s)
	}

	// the account needs a linked character: said once, nothing kept
	site.status = 409
	f, _ = os.OpenFile(logp, os.O_APPEND|os.O_WRONLY, 0o644)
	f.WriteString("10/18 10:00:05.000  [2. Trade] Thoria: again\n")
	f.Close()
	e.Step()
	if s := e.Status(); s.State != Problem || !strings.Contains(s.Message, "/wc link") || len(notes) == 0 {
		t.Fatalf("409 should explain what to do: %+v %v", s, notes)
	}

	// revoked on the website: the key is forgotten
	site.status = 401
	e.lastBeat = e.lastBeat.AddDate(0, 0, -1)
	e.Step()
	if e.Cfg.Token() != "" || e.Status().State != NotConnected {
		t.Fatalf("401 should disconnect: %+v", e.Status())
	}
}
