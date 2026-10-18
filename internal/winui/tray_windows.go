//go:build windows

package winui

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"github.com/spifft/warchest-companion/internal/app"
	"github.com/spifft/warchest-companion/internal/config"
)

// menu commands
const (
	cmdConnect = 100 + iota
	cmdDisconnect
	cmdFolder
	cmdPause
	cmdStartup
	cmdCrossroads
	cmdAccount
	cmdUpdate
	cmdHelp
	cmdQuit
)

type Tray struct {
	Engine     *app.Engine
	Version    string
	DeviceName string
	Repo       string // owner/name on GitHub, for the update check
	Background bool   // started by Windows at sign-in: no welcome box

	hwnd       uintptr
	icon       uintptr
	taskbarMsg uint32
	mu         sync.Mutex
	balloons   [][2]string
	update     string // newer version's page, when there is one
	updateTag  string
}

var current *Tray

// Run shows the tray icon and blocks until the player quits.
func (t *Tray) Run() {
	if !singleInstance() {
		messageBox("Warchest Companion", "Warchest Companion is already running: look for its icon next to the clock (you may need to click the ^ arrow).", mbOK|mbIconInfo)
		return
	}
	runtime.LockOSThread()
	current = t
	t.Engine.OnStatus = func(app.Status) { pPostMessageW.Call(t.hwnd, wmRefresh, 0, 0) }
	t.Engine.Notify = t.queueBalloon
	t.Engine.Open = openURL

	inst, _, _ := pGetModuleHandleW.Call(0)
	cls := u16("WarchestCompanionTray")
	wc := wndClassEx{lpfnWndProc: syscall.NewCallback(wndProc), hInstance: inst, lpszClassName: cls}
	wc.cbSize = uint32(unsafe.Sizeof(wc))
	pRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc)))
	t.hwnd, _, _ = pCreateWindowExW.Call(0, uintptr(unsafe.Pointer(cls)), uintptr(unsafe.Pointer(u16("Warchest Companion"))), 0, 0, 0, 0, 0, hwndMessage, 0, inst, 0)
	r, _, _ := pRegisterWindowMessageW.Call(uintptr(unsafe.Pointer(u16("TaskbarCreated"))))
	t.taskbarMsg = uint32(r)
	t.icon = loadIcon()
	t.addIcon()

	stop := make(chan struct{})
	go t.Engine.Run(stop)
	go t.checkUpdates()
	if !t.Background && t.Engine.Cfg.Token() == "" {
		go func() {
			time.Sleep(600 * time.Millisecond)
			if messageBox("Warchest Companion", "Warchest Companion relays Trade chat from your game to Crossroads on warchestmod.com, so players on the website and in Discord can see it.\n\nOnly Trade chat is ever sent: never whispers, guild, party or anything else.\n\nConnect it to your Warchest account now? (It opens warchestmod.com, where you sign in with Discord.)", mbYesNo|mbIconInfo) == idYes {
				t.connect()
			}
		}()
	}

	var m msg
	for {
		r, _, _ := pGetMessageW.Call(uintptr(unsafe.Pointer(&m)), 0, 0, 0)
		if int32(r) <= 0 {
			break
		}
		pTranslateMessage.Call(uintptr(unsafe.Pointer(&m)))
		pDispatchMessageW.Call(uintptr(unsafe.Pointer(&m)))
	}
	close(stop)
	t.removeIcon()
}

func wndProc(hwnd, message, wParam, lParam uintptr) uintptr {
	t := current
	switch uint32(message) {
	case wmTray:
		if lParam == wmRButtonUp || lParam == wmLButtonUp {
			t.showMenu()
		}
		return 0
	case wmRefresh:
		t.setTip()
		return 0
	case wmBalloon:
		t.showBalloons()
		return 0
	case wmDestroy:
		pPostQuitMessage.Call(0)
		return 0
	}
	if t != nil && t.taskbarMsg != 0 && uint32(message) == t.taskbarMsg {
		t.addIcon() // Explorer restarted: put the icon back
		return 0
	}
	r, _, _ := pDefWindowProcW.Call(hwnd, message, wParam, lParam)
	return r
}

func (t *Tray) nid() *notifyIconData {
	n := &notifyIconData{hWnd: t.hwnd, uID: 1}
	n.cbSize = uint32(unsafe.Sizeof(*n))
	return n
}

func (t *Tray) addIcon() {
	n := t.nid()
	n.uFlags = nifMessage | nifIcon | nifTip
	n.uCallbackMessage = wmTray
	n.hIcon = t.icon
	put(n.szTip[:], t.tipText())
	pShellNotifyIconW.Call(nimAdd, uintptr(unsafe.Pointer(n)))
}

func (t *Tray) removeIcon() { pShellNotifyIconW.Call(nimDelete, uintptr(unsafe.Pointer(t.nid()))) }

func (t *Tray) tipText() string {
	s := t.Engine.Status()
	msg := s.Message
	if msg == "" {
		msg = "Starting…"
	}
	return "Warchest Companion\n" + msg
}

func (t *Tray) setTip() {
	n := t.nid()
	n.uFlags = nifTip
	put(n.szTip[:], t.tipText())
	pShellNotifyIconW.Call(nimModify, uintptr(unsafe.Pointer(n)))
}

func (t *Tray) queueBalloon(title, text string) {
	t.mu.Lock()
	t.balloons = append(t.balloons, [2]string{title, text})
	t.mu.Unlock()
	pPostMessageW.Call(t.hwnd, wmBalloon, 0, 0)
}

func (t *Tray) showBalloons() {
	t.mu.Lock()
	list := t.balloons
	t.balloons = nil
	t.mu.Unlock()
	for _, b := range list {
		n := t.nid()
		n.uFlags = nifInfo
		n.dwInfoFlags = niifInfo
		put(n.szInfoTitle[:], b[0])
		put(n.szInfo[:], b[1])
		pShellNotifyIconW.Call(nimModify, uintptr(unsafe.Pointer(n)))
	}
}

func (t *Tray) showMenu() {
	s := t.Engine.Status()
	cfg := t.Engine.Cfg
	m, _, _ := pCreatePopupMenu.Call()
	defer pDestroyMenu.Call(m)
	add := func(id int, label string, flags uintptr) {
		pAppendMenuW.Call(m, mfString|flags, uintptr(id), uintptr(unsafe.Pointer(u16(label))))
	}
	sep := func() { pAppendMenuW.Call(m, mfSeparator, 0, 0) }

	status := s.Message
	if s.State == app.Relaying && s.Sent > 0 {
		status += " (" + itoa(s.Sent) + " lines)"
	}
	add(0, status, mfGrayed)
	if s.Room != "" {
		add(0, "Room: "+roomLabel(s.Room), mfGrayed)
	}
	sep()
	connected := cfg.Token() != ""
	if !connected {
		label := "Connect to Warchest…"
		if s.State == app.Pairing {
			label = "Connect again (code " + s.Code + " is waiting)…"
		}
		add(cmdConnect, label, 0)
	}
	wow := cfg.WoWDir
	if wow == "" {
		wow = "not set"
	} else if len(wow) > 48 {
		wow = "…" + wow[len(wow)-46:]
	}
	add(cmdFolder, "Game folder: "+wow+"…", 0)
	if connected {
		var f uintptr
		if cfg.Paused {
			f = mfChecked
		}
		add(cmdPause, "Pause relaying", f)
	}
	var f uintptr
	if startsWithWindows() {
		f = mfChecked
	}
	add(cmdStartup, "Start with Windows", f)
	sep()
	add(cmdCrossroads, "Open Crossroads", 0)
	if connected {
		add(cmdAccount, "Manage on warchestmod.com ("+cfg.Account+")", 0)
	}
	add(cmdHelp, "Help", 0)
	t.mu.Lock()
	upd := t.updateTag
	t.mu.Unlock()
	if upd != "" {
		add(cmdUpdate, "Update to "+upd+"…", 0)
	}
	sep()
	if connected {
		add(cmdDisconnect, "Disconnect this computer", 0)
	}
	add(cmdQuit, "Quit (Version "+t.Version+")", 0)

	var pt point
	pGetCursorPos.Call(uintptr(unsafe.Pointer(&pt)))
	pSetForegroundWindow.Call(t.hwnd)
	cmd, _, _ := pTrackPopupMenu.Call(m, tpmReturnCmd|tpmRightBtn, uintptr(pt.x), uintptr(pt.y), 0, t.hwnd, 0)
	t.run(int(cmd))
}

func (t *Tray) run(cmd int) {
	e := t.Engine
	switch cmd {
	case cmdConnect:
		go t.connect()
	case cmdDisconnect:
		if messageBox("Warchest Companion", "Disconnect this computer from your Warchest account? It stops relaying Trade chat until you connect again.", mbYesNo|mbIconWarn) == idYes {
			e.Disconnect()
		}
	case cmdFolder:
		dir := pickFolder(t.hwnd, "Choose your World of Warcraft folder (the one with Logs and WTF in it, or the game's main folder)")
		if dir != "" {
			if err := e.SetWoWDir(dir); err != nil {
				messageBox("Warchest Companion", "That folder didn't work: "+err.Error(), mbOK|mbIconWarn)
			}
			pPostMessageW.Call(t.hwnd, wmRefresh, 0, 0)
		}
	case cmdPause:
		e.SetPaused(!e.Cfg.Paused)
	case cmdStartup:
		exe, _ := os.Executable()
		setStartWithWindows(!startsWithWindows(), exe)
	case cmdCrossroads:
		openURL(e.Client.SiteURL + "/crossroads.html#chat")
	case cmdAccount:
		openURL(e.Client.SiteURL + "/portal.html#/account")
	case cmdHelp:
		openURL(e.Client.SiteURL + "/companion.html")
	case cmdUpdate:
		t.mu.Lock()
		u := t.update
		t.mu.Unlock()
		if u != "" {
			openURL(u)
		}
	case cmdQuit:
		pPostQuitMessage.Call(0)
	}
}

func (t *Tray) connect() {
	if err := t.Engine.Pair(t.DeviceName); err != nil {
		messageBox("Warchest Companion", "Couldn't start connecting: "+err.Error(), mbOK|mbIconWarn)
		return
	}
	s := t.Engine.Status()
	t.queueBalloon("Approve code "+s.Code, "Your browser opened warchestmod.com. Sign in with Discord and approve code "+s.Code+".")
}

// once at start and then daily: is there a newer release on GitHub?
func (t *Tray) checkUpdates() {
	for {
		tag, page, err := t.Engine.Client.Latest(t.Repo)
		if err == nil && newer(tag, t.Version) {
			t.mu.Lock()
			first := t.updateTag != tag
			t.update, t.updateTag = page, tag
			t.mu.Unlock()
			if first && t.Engine.Cfg.SkipVersion != tag {
				t.queueBalloon("Update available", "Warchest Companion "+tag+" is out. Right-click the tray icon to update.")
			}
		}
		time.Sleep(24 * time.Hour)
	}
}

// the tray icon: assets/icon.ico, written next to the settings so Windows can load it
func loadIcon() uintptr {
	p := filepath.Join(config.Dir(), "icon.ico")
	_ = os.MkdirAll(config.Dir(), 0o700)
	if st, err := os.Stat(p); err != nil || st.Size() != int64(len(IconICO)) {
		_ = os.WriteFile(p, IconICO, 0o644)
	}
	h, _, _ := pLoadImageW.Call(0, uintptr(unsafe.Pointer(u16(p))), imageIcon, 0, 0, lrLoadFromFile|lrDefaultSize)
	return h
}

func roomLabel(r string) string {
	p := strings.Split(r, "-")
	if len(p) != 3 {
		return r
	}
	region := map[string]string{"us": "Americas", "eu": "Europe"}[p[0]]
	rules := map[string]string{"normal": "Normal", "pvp": "PvP", "rp": "RP", "hardcore": "Hardcore"}[p[1]]
	fac := map[string]string{"alliance": "Alliance", "horde": "Horde"}[p[2]]
	return region + " · " + rules + " · " + fac
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}
