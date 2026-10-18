// Warchest Companion relays in-game Trade chat to Crossroads on warchestmod.com.
// On Windows it lives in the tray; elsewhere it runs as a console program (for development and testing).
package main

import (
	"flag"
	"os"
	"strings"

	"github.com/spifft/warchest-companion/internal/api"
	"github.com/spifft/warchest-companion/internal/app"
	"github.com/spifft/warchest-companion/internal/config"
)

// set by the release build: -ldflags "-X main.version=1.0.0"
var version = "dev"

const repo = "spifft/warchest-companion"

func main() {
	background := flag.Bool("background", false, "started with Windows: don't show the welcome box")
	wow := flag.String("wow", "", "the World of Warcraft folder (console mode)")
	site := flag.String("site", "", "website address (testing)")
	live := flag.String("live", "", "live service address (testing)")
	db := flag.String("supabase", "", "database address (testing)")
	dbKey := flag.String("supabase-key", "", "database public key (testing)")
	flag.Parse()

	client := api.New(version)
	if *site != "" {
		client.SiteURL = strings.TrimRight(*site, "/")
	}
	if *live != "" {
		client.LiveURL = strings.TrimRight(*live, "/")
	}
	if *db != "" {
		client.SupabaseURL = strings.TrimRight(*db, "/")
	}
	if *dbKey != "" {
		client.SupabaseKey = *dbKey
	}
	cfg := config.Load()
	if *wow != "" {
		cfg.WoWDir = *wow
	}
	name, _ := os.Hostname()
	if name == "" {
		name = "Windows PC"
	}
	e := &app.Engine{Client: client, Cfg: cfg}
	run(e, name, *background)
}
