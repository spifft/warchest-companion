package chatlog

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestParse(t *testing.T) {
	cases := []struct {
		in   string
		ok   bool
		want Line
	}{
		{"10/18 13:29:01.123  [2. Trade - City] Thoria-Forever: WTS [Thorium Ore] 5g each", true, Line{"Trade - City", "Thoria", "WTS [Thorium Ore] 5g each", 0}},
		{"10/18 13:29:01.123  [2. Trade] [Brannoc]: LF enchanter PST", true, Line{"Trade", "Brannoc", "LF enchanter PST", 0}},
		{"10/18 13:29:01  [2. Trade] Brannoc: no milliseconds", true, Line{"Trade", "Brannoc", "no milliseconds", 0}},
		{"10/18 13:29:01.123  [2. Trade] Kaelïth: accents are fine", true, Line{"Trade", "Kaelïth", "accents are fine", 0}},
		// Forever allows spaces in names
		{"10/18 13:29:01.123  [2. Trade - City] [Lan Rat-Forever]: WTS [Thorium Ore]", true, Line{"Trade - City", "Lan Rat", "WTS [Thorium Ore]", 0}},
		{"10/18 13:29:01.123  [2. Trade] Aria Dawnfall: LF healer", true, Line{"Trade", "Aria Dawnfall", "LF healer", 0}},
		{"10/18 13:29:01.123  [2. Trade] Two  Spaces: no", false, Line{}},
		// link codes: item links keep their ID, player and channel links become their names
		{"10/18 13:29:01.123  |Hchannel:channel:2|h[2. Trade - City]|h |Hplayer:Thoria-Forever:12:CHANNEL:2|h[Thoria]|h: WTS |cffffffff|Hitem:10620:0:0:0:0:0:0:0|h[Thorium Ore]|h|r x20",
			true, Line{"Trade - City", "Thoria", "WTS [item:10620] x20", 0}},
		// never relayed
		{"10/18 13:29:01.123  [Brannoc] whispers: secret stuff", false, Line{}},
		{"10/18 13:29:01.123  To [Brannoc]: secret reply", false, Line{}},
		{"10/18 13:29:01.123  [Guild] [Brannoc]: guild talk", false, Line{}},
		{"10/18 13:29:01.123  [Party] Brannoc: party talk", false, Line{}},
		{"10/18 13:29:01.123  [1. General - Ironforge] Brannoc: general chat", false, Line{}},
		{"10/18 13:29:01.123  [4. LookingForGroup] Brannoc: lfg channel", false, Line{}},
		{"10/18 13:29:01.123  Brannoc says: hello", false, Line{}},
		{"10/18 13:29:01.123  You have joined channel: 2. Trade - City", false, Line{}},
		{"10/18 13:29:01.123  [2. Trade] Bad1Name: digits in a name", false, Line{}},
		{"10/18 13:29:01.123  [2. Trade] Brannoc: ", false, Line{}},
		{"garbage", false, Line{}},
	}
	for _, c := range cases {
		got, ok := Parse(c.in)
		got.At = 0 // checked in TestStampTime
		if ok != c.ok || (ok && got != c.want) {
			t.Errorf("Parse(%q) = %+v, %v; want %+v, %v", c.in, got, ok, c.want, c.ok)
		}
	}
}

func TestStampTime(t *testing.T) {
	defer func() { now = time.Now }()
	now = func() time.Time { return time.Date(2026, 10, 18, 18, 0, 0, 0, time.Local) }
	l, ok := Parse("10/18 17:55:15.768  [2. Trade] Grog Bloodeyed: WTS Tigerstrike Mantle 105g")
	want := time.Date(2026, 10, 18, 17, 55, 15, 768*int(time.Millisecond), time.Local).UnixMilli()
	if !ok || l.At != want {
		t.Errorf("At = %d, want %d", l.At, want)
	}
	if l, _ := Parse("10/18 17:55:15  [2. Trade] Grog: no ms"); l.At != want-768 {
		t.Errorf("no milliseconds: At = %d, want %d", l.At, want-768)
	}
	now = func() time.Time { return time.Date(2027, 1, 1, 0, 5, 0, 0, time.Local) }
	if l, _ := Parse("12/31 23:59:00.000  [2. Trade] Grog: happy new year"); l.At != time.Date(2026, 12, 31, 23, 59, 0, 0, time.Local).UnixMilli() {
		t.Errorf("a December line read in January belongs to last year: %d", l.At)
	}
}

func TestTailer(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "WoWChatLog.txt")
	os.WriteFile(p, []byte("10/18 10:00:00.000  [2. Trade] Old: already there before we started\n"), 0o644)
	tl := &Tailer{Path: p}
	if l, _ := tl.Poll(); len(l) != 0 {
		t.Fatalf("first poll should start at the end, got %v", l)
	}
	f, _ := os.OpenFile(p, os.O_APPEND|os.O_WRONLY, 0o644)
	f.WriteString("10/18 10:00:01.000  [2. Trade] Thoria: one\r\n10/18 10:00:02.000  [2. Trade] Thoria: tw")
	f.Close()
	l, _ := tl.Poll()
	if len(l) != 1 || l[0] != "10/18 10:00:01.000  [2. Trade] Thoria: one" {
		t.Fatalf("want one complete line, got %q", l)
	}
	f, _ = os.OpenFile(p, os.O_APPEND|os.O_WRONLY, 0o644)
	f.WriteString("o\n")
	f.Close()
	if l, _ = tl.Poll(); len(l) != 1 || l[0] != "10/18 10:00:02.000  [2. Trade] Thoria: two" {
		t.Fatalf("want the finished partial line, got %q", l)
	}
	// the game starts a new log: read it from the beginning
	os.WriteFile(p, []byte("10/18 11:00:00.000  [2. Trade] New: fresh file\n"), 0o644)
	if l, _ = tl.Poll(); len(l) != 1 || l[0] != "10/18 11:00:00.000  [2. Trade] New: fresh file" {
		t.Fatalf("want the new file's line, got %q", l)
	}
}
