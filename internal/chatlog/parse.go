// Package chatlog reads World of Warcraft's chat log (Logs/WoWChatLog.txt, written while /chatlog is on)
// and picks out Trade channel lines. Everything else in the log (whispers, guild, party, say, system
// messages) is ignored here and never leaves the computer.
package chatlog

import (
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Line is one Trade chat message.
type Line struct {
	Channel string `json:"ch"` // "Trade" or "Trade - City"
	Name    string `json:"n"`  // the character who said it, without a realm
	Text    string `json:"t"`  // the message; item links become [item:ID] when the log has their codes
}

var (
	// "10/18 13:29:01.123  " (and older clients without milliseconds)
	stamp = regexp.MustCompile(`^\d{1,2}/\d{1,2}(?:/\d{2,4})? +\d{1,2}:\d{2}:\d{2}(?:\.\d{1,3})? +`)
	// "[2. Trade - City] " or "[2. Trade] "
	channel = regexp.MustCompile(`^\[(\d{1,2})\. ([^\]]{2,40})\] `)
	// a hyperlink: |Hitem:2772:0:0:0|h[Thorium Ore]|h  (|Hplayer:Name-Realm|h[Name]|h, |Hchannel:...|h[2. Trade]|h)
	hyperlink = regexp.MustCompile(`\|H([a-z]+):([^|]*)\|h(.*?)\|h`)
	colour    = regexp.MustCompile(`\|c[0-9a-fA-F]{8}|\|r`)
	textures  = regexp.MustCompile(`\|T[^|]*\|t|\|A[^|]*\|a`)
	itemID    = regexp.MustCompile(`^(\d{1,7})`)
)

// clean turns the game's link and colour codes into plain text; item links become [item:ID].
func clean(s string) string {
	s = hyperlink.ReplaceAllStringFunc(s, func(m string) string {
		p := hyperlink.FindStringSubmatch(m)
		kind, data, shown := p[1], p[2], p[3]
		if kind == "item" {
			if id := itemID.FindString(data); id != "" {
				return "[item:" + id + "]"
			}
		}
		return shown
	})
	s = colour.ReplaceAllString(s, "")
	s = textures.ReplaceAllString(s, "")
	return s
}

// Parse reads one line of the chat log. ok is false for anything that isn't a Trade channel message.
func Parse(raw string) (Line, bool) {
	raw = strings.TrimRight(raw, "\r\n")
	if !utf8.ValidString(raw) {
		return Line{}, false
	}
	loc := stamp.FindStringIndex(raw)
	if loc == nil {
		return Line{}, false
	}
	rest := clean(raw[loc[1]:])
	m := channel.FindStringSubmatchIndex(rest)
	if m == nil {
		return Line{}, false
	}
	ch := rest[m[4]:m[5]]
	if !strings.HasPrefix(strings.ToLower(ch), "trade") {
		return Line{}, false
	}
	rest = rest[m[1]:]
	// the speaker: "[Name-Realm]: text" or "Name: text"
	var name, text string
	if strings.HasPrefix(rest, "[") {
		end := strings.Index(rest, "]: ")
		if end < 0 {
			return Line{}, false
		}
		name, text = rest[1:end], rest[end+3:]
	} else {
		end := strings.Index(rest, ": ")
		if end < 0 {
			return Line{}, false
		}
		name, text = rest[:end], rest[end+2:]
	}
	if i := strings.IndexByte(name, '-'); i > 0 {
		name = name[:i] // Name-Realm -> Name
	}
	if !validName(name) {
		return Line{}, false
	}
	text = strings.TrimSpace(text)
	if text == "" || len(text) > 600 {
		return Line{}, false
	}
	return Line{Channel: ch, Name: name, Text: text}, true
}

// a character name: letters (accents allowed), and on WoW: Forever single spaces between words ("Lan Rat"), 2 to 24 long
func validName(n string) bool {
	c := utf8.RuneCountInString(n)
	if c < 2 || c > 24 || strings.HasPrefix(n, " ") || strings.HasSuffix(n, " ") || strings.Contains(n, "  ") {
		return false
	}
	for _, r := range n {
		if !unicode.IsLetter(r) && r != ' ' {
			return false
		}
	}
	return true
}
