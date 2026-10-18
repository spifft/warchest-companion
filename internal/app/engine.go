// Package app is the companion's engine: pairing, following the chat log and relaying Trade lines.
// The Windows tray (internal/winui) and the console mode for testing both drive it.
package app

import (
	"errors"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/spifft/warchest-companion/internal/api"
	"github.com/spifft/warchest-companion/internal/chatlog"
	"github.com/spifft/warchest-companion/internal/config"
	"github.com/spifft/warchest-companion/internal/wowdir"
)

type State string

const (
	NotConnected State = "notconnected" // no device key yet
	Pairing      State = "pairing"      // waiting for the player to approve the code on the website
	NoWoW        State = "nowow"        // the game folder isn't set or wasn't found
	Waiting      State = "waiting"      // connected, no chat log yet (type /chatlog in game)
	Relaying     State = "relaying"
	Paused       State = "paused"
	Problem      State = "problem" // something the player has to fix (message says what)
)

type Status struct {
	State    State
	Message  string
	Code     string // the pairing code while Pairing
	Account  string
	Char     string
	Room     string
	Sent     int       // Trade lines relayed since the app started
	LastLine time.Time // when the chat log last had a Trade line
}

type Engine struct {
	Client *api.Client
	Cfg    *config.Config
	// OnStatus is called whenever the status changes; Notify for things worth a pop-up; Open opens a web page.
	OnStatus func(Status)
	Notify   func(title, msg string)
	Open     func(url string)
	Interval time.Duration

	mu        sync.Mutex
	st        Status
	secret    string
	pairUntil time.Time
	tail      *chatlog.Tailer
	queue     []chatlog.Line
	nextSend  time.Time
	lastBeat  time.Time
	warnedLog bool
}

func (e *Engine) Status() Status { e.mu.Lock(); defer e.mu.Unlock(); return e.st }

func (e *Engine) set(f func(*Status)) {
	e.mu.Lock()
	before := e.st
	f(&e.st)
	now := e.st
	e.mu.Unlock()
	if e.OnStatus != nil && now != before {
		e.OnStatus(now)
	}
}

func (e *Engine) notify(title, msg string) {
	if e.Notify != nil {
		e.Notify(title, msg)
	}
}

// Pair starts connecting this computer to a Warchest account: it gets a code and opens the approval page.
func (e *Engine) Pair(deviceName string) error {
	secret := api.NewSecret()
	p, err := e.Client.Start(secret, deviceName)
	if err != nil {
		e.set(func(s *Status) { s.State, s.Message = Problem, err.Error() })
		return err
	}
	e.mu.Lock()
	e.secret, e.pairUntil = secret, time.Now().Add(14*time.Minute)
	e.mu.Unlock()
	e.set(func(s *Status) {
		s.State, s.Code, s.Message = Pairing, p.Code, "Approve code "+p.Code+" on warchestmod.com"
	})
	if e.Open != nil {
		e.Open(e.Client.PairURL(p.Code))
	}
	return nil
}

// Disconnect forgets the device key (the player can also revoke it on their Account page).
func (e *Engine) Disconnect() {
	_ = e.Cfg.SetToken("")
	e.Cfg.Account = ""
	_ = e.Cfg.Save()
	e.mu.Lock()
	e.secret, e.queue = "", nil
	e.mu.Unlock()
	e.set(func(s *Status) { *s = Status{State: NotConnected, Message: "Not connected to Warchest"} })
}

func (e *Engine) SetWoWDir(dir string) error {
	if _, ok := wowdir.LogPath(dir); !ok {
		return errors.New("that folder doesn't look like World of Warcraft (pick the folder with Logs and WTF in it)")
	}
	e.Cfg.WoWDir = dir
	if err := e.Cfg.Save(); err != nil {
		return err
	}
	e.mu.Lock()
	e.tail = nil
	e.mu.Unlock()
	return nil
}

func (e *Engine) SetPaused(p bool) {
	e.Cfg.Paused = p
	_ = e.Cfg.Save()
	e.mu.Lock()
	e.queue = nil
	if e.tail != nil {
		e.tail = nil // when resuming, start from the end again (nothing said while paused is sent)
	}
	e.mu.Unlock()
}

// Run does one step every Interval until stop is closed.
func (e *Engine) Run(stop <-chan struct{}) {
	if e.Interval == 0 {
		e.Interval = 3 * time.Second
	}
	if e.Cfg.WoWDir == "" {
		if g := wowdir.Guess(); g != "" {
			e.Cfg.WoWDir = g
			_ = e.Cfg.Save()
		}
	}
	t := time.NewTicker(e.Interval)
	defer t.Stop()
	e.Step()
	for {
		select {
		case <-stop:
			return
		case <-t.C:
			e.Step()
		}
	}
}

// Step is one round: finish pairing, read new chat, send what's waiting.
func (e *Engine) Step() {
	e.mu.Lock()
	secret, until := e.secret, e.pairUntil
	e.mu.Unlock()
	if secret != "" {
		e.stepPairing(secret, until)
		return
	}
	tok := e.Cfg.Token()
	if tok == "" {
		e.set(func(s *Status) {
			if s.State != Problem {
				*s = Status{State: NotConnected, Message: "Not connected to Warchest"}
			}
		})
		return
	}
	if e.Cfg.Paused {
		e.set(func(s *Status) { s.State, s.Message = Paused, "Paused: Trade chat isn't being relayed" })
		return
	}
	logPath, ok := wowdir.LogPath(e.Cfg.WoWDir)
	if !ok {
		e.set(func(s *Status) { s.State, s.Message = NoWoW, "Choose your World of Warcraft folder" })
		return
	}
	e.readLog(logPath)
	e.send(tok)
}

func (e *Engine) stepPairing(secret string, until time.Time) {
	if time.Now().After(until) {
		e.mu.Lock()
		e.secret = ""
		e.mu.Unlock()
		e.set(func(s *Status) { *s = Status{State: NotConnected, Message: "The code expired. Connect again."} })
		return
	}
	c, err := e.Client.Claim(secret)
	if err != nil {
		return // try again next round
	}
	switch c.Status {
	case "ok":
		if err := e.Cfg.SetToken(c.Token); err != nil {
			e.set(func(s *Status) { s.State, s.Message = Problem, "Couldn't save the key: "+err.Error() })
			return
		}
		e.Cfg.Account = c.Account
		_ = e.Cfg.Save()
		e.mu.Lock()
		e.secret, e.lastBeat = "", time.Time{}
		e.mu.Unlock()
		char := ""
		if c.Char != nil {
			char = c.Char.Name
		}
		e.set(func(s *Status) { *s = Status{State: Waiting, Account: c.Account, Char: char, Message: "Connected"} })
		e.notify("Connected to Warchest", "This computer now relays Trade chat for "+c.Account+". Keep /chatlog on in game.")
	case "expired":
		e.mu.Lock()
		e.secret = ""
		e.mu.Unlock()
		e.set(func(s *Status) { *s = Status{State: NotConnected, Message: "The code expired. Connect again."} })
	}
}

func (e *Engine) readLog(path string) {
	e.mu.Lock()
	if e.tail == nil || e.tail.Path != path {
		e.tail = &chatlog.Tailer{Path: path}
	}
	tl := e.tail
	e.mu.Unlock()
	raw, err := tl.Poll()
	if err != nil {
		if os.IsNotExist(err) {
			e.set(func(s *Status) {
				if s.State != Problem {
					s.State, s.Message = Waiting, "Waiting for the chat log: type /chatlog in game"
				}
			})
			return
		}
		e.set(func(s *Status) { s.State, s.Message = Problem, "Can't read the chat log: "+err.Error() })
		return
	}
	var got []chatlog.Line
	for _, r := range raw {
		if l, ok := chatlog.Parse(r); ok {
			got = append(got, l)
		}
	}
	e.mu.Lock()
	e.queue = append(e.queue, got...)
	if len(e.queue) > 400 { // offline for a while: keep the newest
		e.queue = e.queue[len(e.queue)-400:]
	}
	e.mu.Unlock()
	if len(got) > 0 {
		e.set(func(s *Status) { s.LastLine = time.Now() })
	}
}

func (e *Engine) send(tok string) {
	e.mu.Lock()
	if time.Now().Before(e.nextSend) {
		e.mu.Unlock()
		return
	}
	n := len(e.queue)
	if n > 40 {
		n = 40
	}
	batch := append([]chatlog.Line(nil), e.queue[:n]...)
	beat := time.Since(e.lastBeat) > 60*time.Second
	e.mu.Unlock()
	if len(batch) == 0 && !beat {
		return
	}
	res, err := e.Client.Relay(tok, batch)
	if err != nil {
		var ae *api.Error
		switch {
		case api.Disconnected(err):
			_ = e.Cfg.SetToken("")
			_ = e.Cfg.Save()
			e.mu.Lock()
			e.queue = nil
			e.mu.Unlock()
			e.set(func(s *Status) { *s = Status{State: NotConnected, Message: err.Error()} })
			e.notify("Warchest Companion", err.Error())
		case errors.As(err, &ae) && ae.Status == 429:
			wait := ae.Retry
			if wait <= 0 {
				wait = 30
			}
			e.mu.Lock()
			e.nextSend = time.Now().Add(time.Duration(wait) * time.Second)
			e.mu.Unlock()
		case errors.As(err, &ae) && (ae.Status == 409 || ae.Status == 403):
			// the account needs something (a character, /wc link): say so once, drop what's queued, check again in a minute
			e.mu.Lock()
			e.queue, e.lastBeat = nil, time.Now()
			first := !e.warnedLog
			e.warnedLog = true
			e.mu.Unlock()
			e.set(func(s *Status) { s.State, s.Message = Problem, ae.Message })
			if first {
				e.notify("Warchest Companion", ae.Message)
			}
		default:
			e.mu.Lock()
			e.nextSend = time.Now().Add(15 * time.Second) // offline: keep the lines and try again
			waiting := len(e.queue)
			e.mu.Unlock()
			e.set(func(s *Status) {
				s.Message = fmt.Sprintf("Can't reach Crossroads right now; %d lines waiting", waiting)
			})
		}
		return
	}
	e.mu.Lock()
	e.queue = e.queue[len(batch):]
	e.lastBeat, e.warnedLog = time.Now(), false
	e.mu.Unlock()
	e.set(func(s *Status) {
		s.Char, s.Room, s.Sent = res.Char, res.Room, s.Sent+len(batch)
		if !s.LastLine.IsZero() && time.Since(s.LastLine) < 15*time.Minute {
			s.State, s.Message = Relaying, "Relaying Trade chat as "+res.Char
		} else {
			s.State, s.Message = Waiting, "Connected as "+res.Char+". No Trade chat yet: is /chatlog on in game?"
		}
	})
}
