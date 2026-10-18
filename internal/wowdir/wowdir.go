// Package wowdir finds the chat log (Logs\WoWChatLog.txt) from a game folder.
package wowdir

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const LogName = "WoWChatLog.txt"

// LogPath returns where the chat log is (or will be) for a folder the player picked: the game folder,
// a client folder inside it (_classic_, _classic_era_, ...), or the Logs folder itself. ok is false
// when the folder doesn't look like a WoW install.
func LogPath(dir string) (string, bool) {
	if dir == "" {
		return "", false
	}
	if strings.EqualFold(filepath.Base(dir), "Logs") && isDir(dir) {
		return filepath.Join(dir, LogName), true
	}
	if isDir(filepath.Join(dir, "Logs")) && (exists(filepath.Join(dir, "WTF")) || exists(filepath.Join(dir, "Interface")) || exists(filepath.Join(dir, "Logs", LogName))) {
		return filepath.Join(dir, "Logs", LogName), true
	}
	// a launcher folder with client folders inside: prefer one whose chat log changed most recently
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", false
	}
	type cand struct {
		path string
		mod  int64
	}
	var cs []cand
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		sub := filepath.Join(dir, e.Name())
		if isDir(filepath.Join(sub, "Logs")) && (exists(filepath.Join(sub, "WTF")) || exists(filepath.Join(sub, "Interface"))) {
			p := filepath.Join(sub, "Logs", LogName)
			var mod int64
			if st, err := os.Stat(p); err == nil {
				mod = st.ModTime().Unix()
			}
			cs = append(cs, cand{p, mod})
		}
	}
	if len(cs) == 0 {
		return "", false
	}
	sort.Slice(cs, func(i, j int) bool { return cs[i].mod > cs[j].mod })
	return cs[0].path, true
}

// Guess looks in the usual places; the player can always pick the folder instead.
func Guess() string {
	var roots []string
	for _, env := range []string{"ProgramFiles(x86)", "ProgramFiles", "PUBLIC"} {
		if v := os.Getenv(env); v != "" {
			roots = append(roots, v)
		}
	}
	for _, d := range []string{"C:\\", "D:\\", "E:\\"} {
		roots = append(roots, d, filepath.Join(d, "Games"))
	}
	names := []string{"World of Warcraft Forever", "WoW Forever", "WowForever", "World of Warcraft"}
	for _, r := range roots {
		for _, n := range names {
			p := filepath.Join(r, n)
			if _, ok := LogPath(p); ok {
				return p
			}
		}
	}
	return ""
}

func isDir(p string) bool  { st, err := os.Stat(p); return err == nil && st.IsDir() }
func exists(p string) bool { _, err := os.Stat(p); return err == nil }
