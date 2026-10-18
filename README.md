# Warchest Companion

A small Windows app that relays **Trade chat** from World of Warcraft: Forever to [Crossroads](https://warchestmod.com/crossroads.html), so players on the website and in Discord can see it and answer.

**[Download the latest version](https://github.com/spifft/warchest-companion/releases/latest)** · [Help](https://warchestmod.com/companion.html)

## What it does

1. You connect it to your Warchest account once. It shows a code, you approve it on warchestmod.com, and it gets its own key. No password ever goes into the app.
2. In game, chat logging is on (`/chatlog`), so the game writes chat to `Logs\WoWChatLog.txt`.
3. The app reads new lines from that file every few seconds and sends **only Trade channel messages** (who said it, what they said) to Crossroads, into your character's room.

## What it never sends

Whispers, guild, officer, party, raid, say, other channels, or anything else on your computer. Lines are picked out in [`internal/chatlog/parse.go`](internal/chatlog/parse.go) (with [tests](internal/chatlog/chatlog_test.go) for exactly that), and Crossroads checks the channel again.

## Is it safe?

- **Open source, no dependencies:** only Go's standard library and Windows' own APIs. Everything it does is in this repository.
- **Signed releases,** built by GitHub Actions from this code ([workflow](.github/workflows/release.yml)), with a SHA-256 checksum on every release.
- **Stays in its lane:** it only reads the chat log file, and only talks to warchestmod.com and to GitHub (to check for updates). It doesn't touch the game, inject anything, or read memory.
- **Its key is protected** with Windows' per-user encryption (DPAPI) in `%APPDATA%\Warchest Companion`. You can disconnect a computer any time on your [Account page](https://warchestmod.com/portal.html#/account).

## Building it yourself

```
go test ./...
GOOS=windows GOARCH=amd64 go build -ldflags "-H=windowsgui" -o WarchestCompanion.exe ./cmd/companion
```

On other systems it runs as a console program (for development): `go run ./cmd/companion --wow /path/to/WoW`.

## License

MIT, see [LICENSE](LICENSE). Not affiliated with Blizzard Entertainment.
