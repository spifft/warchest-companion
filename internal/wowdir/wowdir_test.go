package wowdir

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func mk(t *testing.T, parts ...string) string {
	p := filepath.Join(parts...)
	if err := os.MkdirAll(p, 0o755); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestLogPath(t *testing.T) {
	root := t.TempDir()
	// a launcher folder with two clients; the one played most recently wins
	mk(t, root, "_classic_", "Logs")
	mk(t, root, "_classic_", "WTF")
	mk(t, root, "_classic_era_", "Logs")
	mk(t, root, "_classic_era_", "Interface")
	old := filepath.Join(root, "_classic_", "Logs", LogName)
	recent := filepath.Join(root, "_classic_era_", "Logs", LogName)
	os.WriteFile(old, []byte("x"), 0o644)
	os.WriteFile(recent, []byte("x"), 0o644)
	os.Chtimes(old, time.Now().Add(-48*time.Hour), time.Now().Add(-48*time.Hour))
	if p, ok := LogPath(root); !ok || p != recent {
		t.Fatalf("launcher folder: got %q %v, want %q", p, ok, recent)
	}
	// the client folder itself, and its Logs folder
	if p, ok := LogPath(filepath.Join(root, "_classic_")); !ok || p != old {
		t.Fatalf("client folder: got %q %v", p, ok)
	}
	if p, ok := LogPath(filepath.Join(root, "_classic_", "Logs")); !ok || p != old {
		t.Fatalf("Logs folder: got %q %v", p, ok)
	}
	// somewhere unrelated
	if _, ok := LogPath(mk(t, root, "Documents")); ok {
		t.Fatal("an unrelated folder should not count")
	}
}
