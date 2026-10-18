// Package api talks to warchestmod.com: pairing this computer with a Warchest account, and relaying Trade lines.
package api

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/spifft/warchest-companion/internal/chatlog"
)

// The website's public database address and its public (anon) key: the same ones every visitor's browser uses.
const (
	DefaultSupabaseURL = "https://yfezdbzwsrkedgpzzjob.supabase.co"
	DefaultSupabaseKey = "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJpc3MiOiJzdXBhYmFzZSIsInJlZiI6InlmZXpkYnp3c3JrZWRncHp6am9iIiwicm9sZSI6ImFub24iLCJpYXQiOjE3OTIwODQxNjQsImV4cCI6MjEwNzY2MDE2NH0.mg0WIeLKDsd6zGlSzW5uCytCpBvs7xcqkJeOL8x8tTY"
	DefaultLiveURL     = "https://warchest-crossroads.kbarlowd.workers.dev"
	DefaultSiteURL     = "https://warchestmod.com"
)

type Client struct {
	SupabaseURL, SupabaseKey, LiveURL, SiteURL string
	UserAgent                                  string
	HTTP                                       *http.Client
}

func New(version string) *Client {
	return &Client{SupabaseURL: DefaultSupabaseURL, SupabaseKey: DefaultSupabaseKey, LiveURL: DefaultLiveURL, SiteURL: DefaultSiteURL,
		UserAgent: "WarchestCompanion/" + version, HTTP: &http.Client{Timeout: 20 * time.Second}}
}

// Error is a problem the server explained (shown to the player as is).
type Error struct {
	Status  int
	Code    string
	Message string
	Retry   int
}

func (e *Error) Error() string { return e.Message }

// NewSecret makes the random secret this computer keeps while pairing.
func NewSecret() string {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

func (c *Client) rpc(fn string, args any, out any) error {
	body, _ := json.Marshal(args)
	req, _ := http.NewRequest("POST", c.SupabaseURL+"/rest/v1/rpc/"+fn, bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("apikey", c.SupabaseKey)
	req.Header.Set("Authorization", "Bearer "+c.SupabaseKey)
	req.Header.Set("User-Agent", c.UserAgent)
	res, err := c.HTTP.Do(req)
	if err != nil {
		return fmt.Errorf("can't reach warchestmod.com: %w", err)
	}
	defer res.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if res.StatusCode >= 300 {
		var e struct {
			Message string `json:"message"`
		}
		_ = json.Unmarshal(data, &e)
		if e.Message == "" {
			e.Message = "warchestmod.com answered " + res.Status
		}
		return &Error{Status: res.StatusCode, Message: e.Message}
	}
	return json.Unmarshal(data, out)
}

type Pairing struct {
	Code    string    `json:"code"`
	Expires time.Time `json:"expires"`
}

// Start asks for a pairing code; the player approves it on the website.
func (c *Client) Start(secret, deviceName string) (*Pairing, error) {
	var p Pairing
	if err := c.rpc("companion_start", map[string]string{"p_secret": secret, "p_name": deviceName}, &p); err != nil {
		return nil, err
	}
	return &p, nil
}

// PairURL is the page where the player approves the code.
func (c *Client) PairURL(code string) string { return c.SiteURL + "/portal.html#/companion/" + code }

type Claim struct {
	Status  string `json:"status"` // waiting | ok | expired
	Token   string `json:"token"`
	Account string `json:"account"`
	Char    *struct {
		Name     string `json:"n"`
		Verified bool   `json:"v"`
		Room     string `json:"room"`
	} `json:"char"`
}

// Claim picks up the device key once the player approved the code.
func (c *Client) Claim(secret string) (*Claim, error) {
	var cl Claim
	if err := c.rpc("companion_claim", map[string]string{"p_secret": secret}, &cl); err != nil {
		return nil, err
	}
	return &cl, nil
}

type RelayResult struct {
	OK    bool   `json:"ok"`
	Added int    `json:"added"`
	Dup   int    `json:"dup"`
	Room  string `json:"room"`
	Char  string `json:"char"`
}

// Relay sends Trade lines (only ever chatlog.Line values, which are Trade by construction).
func (c *Client) Relay(token string, lines []chatlog.Line) (*RelayResult, error) {
	if lines == nil {
		lines = []chatlog.Line{}
	}
	body, _ := json.Marshal(map[string]any{"lines": lines})
	req, _ := http.NewRequest("POST", c.LiveURL+"/relay", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("User-Agent", c.UserAgent)
	res, err := c.HTTP.Do(req)
	if err != nil {
		return nil, fmt.Errorf("can't reach Crossroads: %w", err)
	}
	defer res.Body.Close()
	data, _ := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if res.StatusCode >= 300 {
		var e struct {
			Error string `json:"error"`
			Code  string `json:"code"`
			Retry int    `json:"retry"`
		}
		_ = json.Unmarshal(data, &e)
		if e.Error == "" {
			e.Error = "Crossroads answered " + res.Status
		}
		return nil, &Error{Status: res.StatusCode, Code: e.Code, Message: e.Error, Retry: e.Retry}
	}
	var r RelayResult
	if err := json.Unmarshal(data, &r); err != nil {
		return nil, err
	}
	return &r, nil
}

// Disconnected reports whether the server says this computer's key no longer works.
func Disconnected(err error) bool {
	var e *Error
	return errors.As(err, &e) && e.Status == 401
}

// Latest asks GitHub for the newest release ("v0.2.0") and its page.
func (c *Client) Latest(repo string) (tag, page string, err error) {
	req, _ := http.NewRequest("GET", "https://api.github.com/repos/"+repo+"/releases/latest", nil)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", c.UserAgent)
	res, err := c.HTTP.Do(req)
	if err != nil {
		return "", "", err
	}
	defer res.Body.Close()
	if res.StatusCode != 200 {
		return "", "", fmt.Errorf("GitHub answered %s", res.Status)
	}
	var r struct {
		Tag  string `json:"tag_name"`
		Page string `json:"html_url"`
	}
	if err := json.NewDecoder(io.LimitReader(res.Body, 1<<20)).Decode(&r); err != nil {
		return "", "", err
	}
	return r.Tag, r.Page, nil
}
