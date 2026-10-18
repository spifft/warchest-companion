//go:build windows

// Package winui is the companion's Windows face: a tray icon with a menu, pop-up notifications,
// a folder picker and "Start with Windows". It uses only Windows' own APIs (no third-party code).
package winui

import (
	"syscall"
	"unsafe"
)

var (
	user32   = syscall.NewLazyDLL("user32.dll")
	shell32  = syscall.NewLazyDLL("shell32.dll")
	kernel32 = syscall.NewLazyDLL("kernel32.dll")
	ole32    = syscall.NewLazyDLL("ole32.dll")
	advapi32 = syscall.NewLazyDLL("advapi32.dll")

	pRegisterClassExW       = user32.NewProc("RegisterClassExW")
	pCreateWindowExW        = user32.NewProc("CreateWindowExW")
	pDefWindowProcW         = user32.NewProc("DefWindowProcW")
	pGetMessageW            = user32.NewProc("GetMessageW")
	pTranslateMessage       = user32.NewProc("TranslateMessage")
	pDispatchMessageW       = user32.NewProc("DispatchMessageW")
	pPostMessageW           = user32.NewProc("PostMessageW")
	pPostQuitMessage        = user32.NewProc("PostQuitMessage")
	pCreatePopupMenu        = user32.NewProc("CreatePopupMenu")
	pAppendMenuW            = user32.NewProc("AppendMenuW")
	pTrackPopupMenu         = user32.NewProc("TrackPopupMenu")
	pDestroyMenu            = user32.NewProc("DestroyMenu")
	pGetCursorPos           = user32.NewProc("GetCursorPos")
	pSetForegroundWindow    = user32.NewProc("SetForegroundWindow")
	pMessageBoxW            = user32.NewProc("MessageBoxW")
	pLoadImageW             = user32.NewProc("LoadImageW")
	pRegisterWindowMessageW = user32.NewProc("RegisterWindowMessageW")
	pShellNotifyIconW       = shell32.NewProc("Shell_NotifyIconW")
	pShellExecuteW          = shell32.NewProc("ShellExecuteW")
	pSHBrowseForFolderW     = shell32.NewProc("SHBrowseForFolderW")
	pSHGetPathFromIDListW   = shell32.NewProc("SHGetPathFromIDListW")
	pGetModuleHandleW       = kernel32.NewProc("GetModuleHandleW")
	pCreateMutexW           = kernel32.NewProc("CreateMutexW")
	pCoInitializeEx         = ole32.NewProc("CoInitializeEx")
	pCoTaskMemFree          = ole32.NewProc("CoTaskMemFree")
	pRegCreateKeyExW        = advapi32.NewProc("RegCreateKeyExW")
	pRegSetValueExW         = advapi32.NewProc("RegSetValueExW")
	pRegDeleteValueW        = advapi32.NewProc("RegDeleteValueW")
	pRegQueryValueExW       = advapi32.NewProc("RegQueryValueExW")
	pRegCloseKey            = advapi32.NewProc("RegCloseKey")
)

const (
	wmDestroy        = 0x0002
	wmCommand        = 0x0111
	wmLButtonUp      = 0x0202
	wmRButtonUp      = 0x0205
	wmApp            = 0x8000
	wmTray           = wmApp + 1 // tray icon events
	wmRefresh        = wmApp + 2 // the engine's status changed
	wmBalloon        = wmApp + 3 // show a queued notification
	nimAdd           = 0
	nimModify        = 1
	nimDelete        = 2
	nifMessage       = 0x1
	nifIcon          = 0x2
	nifTip           = 0x4
	nifInfo          = 0x10
	niifInfo         = 0x1
	mfString         = 0x0
	mfGrayed         = 0x1
	mfChecked        = 0x8
	mfSeparator      = 0x800
	tpmReturnCmd     = 0x100
	tpmRightBtn      = 0x2
	imageIcon        = 1
	lrLoadFromFile   = 0x10
	lrDefaultSize    = 0x40
	mbOK             = 0x0
	mbIconInfo       = 0x40
	mbIconWarn       = 0x30
	mbYesNo          = 0x4
	idYes            = 6
	hwndMessage      = ^uintptr(2) // HWND_MESSAGE (-3)
	hkeyCurrentUser  = 0x80000001
	keySetValue      = 0x0002
	keyQueryValue    = 0x0001
	regSZ            = 1
	errAlreadyExists = 183
)

type wndClassEx struct {
	cbSize        uint32
	style         uint32
	lpfnWndProc   uintptr
	cbClsExtra    int32
	cbWndExtra    int32
	hInstance     uintptr
	hIcon         uintptr
	hCursor       uintptr
	hbrBackground uintptr
	lpszMenuName  *uint16
	lpszClassName *uint16
	hIconSm       uintptr
}

type point struct{ x, y int32 }

type msg struct {
	hwnd    uintptr
	message uint32
	wParam  uintptr
	lParam  uintptr
	time    uint32
	pt      point
	private uint32
}

type notifyIconData struct {
	cbSize           uint32
	hWnd             uintptr
	uID              uint32
	uFlags           uint32
	uCallbackMessage uint32
	hIcon            uintptr
	szTip            [128]uint16
	dwState          uint32
	dwStateMask      uint32
	szInfo           [256]uint16
	uVersion         uint32
	szInfoTitle      [64]uint16
	dwInfoFlags      uint32
	guidItem         [16]byte
	hBalloonIcon     uintptr
}

type browseInfo struct {
	hwndOwner      uintptr
	pidlRoot       uintptr
	pszDisplayName *uint16
	lpszTitle      *uint16
	ulFlags        uint32
	lpfn           uintptr
	lParam         uintptr
	iImage         int32
}

func u16(s string) *uint16 { p, _ := syscall.UTF16PtrFromString(s); return p }

// copy a Go string into a fixed UTF-16 field, cut to fit
func put(dst []uint16, s string) {
	w, _ := syscall.UTF16FromString(s)
	if len(w) > len(dst) {
		w = w[:len(dst)]
		w[len(w)-1] = 0
	}
	copy(dst, w)
}

func messageBox(title, text string, flags uintptr) int {
	r, _, _ := pMessageBoxW.Call(0, uintptr(unsafe.Pointer(u16(text))), uintptr(unsafe.Pointer(u16(title))), flags)
	return int(r)
}

func openURL(url string) {
	pShellExecuteW.Call(0, uintptr(unsafe.Pointer(u16("open"))), uintptr(unsafe.Pointer(u16(url))), 0, 0, 1)
}

// pickFolder shows Windows' folder picker; "" when cancelled
func pickFolder(owner uintptr, title string) string {
	pCoInitializeEx.Call(0, 0x2) // apartment-threaded, needed by the picker
	buf := make([]uint16, 260)
	bi := browseInfo{hwndOwner: owner, pszDisplayName: &buf[0], lpszTitle: u16(title), ulFlags: 0x1 | 0x40} // BIF_RETURNONLYFSDIRS | BIF_NEWDIALOGSTYLE
	pidl, _, _ := pSHBrowseForFolderW.Call(uintptr(unsafe.Pointer(&bi)))
	if pidl == 0 {
		return ""
	}
	defer pCoTaskMemFree.Call(pidl)
	path := make([]uint16, 1024)
	if r, _, _ := pSHGetPathFromIDListW.Call(pidl, uintptr(unsafe.Pointer(&path[0]))); r == 0 {
		return ""
	}
	return syscall.UTF16ToString(path)
}

// singleInstance returns false if the companion is already running for this Windows user
func singleInstance() bool {
	_, _, err := pCreateMutexW.Call(0, 0, uintptr(unsafe.Pointer(u16("Local\\WarchestCompanion"))))
	if errno, ok := err.(syscall.Errno); ok && errno == errAlreadyExists {
		return false
	}
	return true
}

/* ---------- Start with Windows (HKCU\...\Run) ---------- */
const runKey = `Software\Microsoft\Windows\CurrentVersion\Run`
const runName = "Warchest Companion"

func openRun(access uintptr) (uintptr, bool) {
	var key uintptr
	r, _, _ := pRegCreateKeyExW.Call(hkeyCurrentUser, uintptr(unsafe.Pointer(u16(runKey))), 0, 0, 0, access, 0, uintptr(unsafe.Pointer(&key)), 0)
	return key, r == 0
}

func startsWithWindows() bool {
	key, ok := openRun(keyQueryValue)
	if !ok {
		return false
	}
	defer pRegCloseKey.Call(key)
	var size uint32
	r, _, _ := pRegQueryValueExW.Call(key, uintptr(unsafe.Pointer(u16(runName))), 0, 0, 0, uintptr(unsafe.Pointer(&size)))
	return r == 0 && size > 0
}

func setStartWithWindows(on bool, exe string) {
	key, ok := openRun(keySetValue)
	if !ok {
		return
	}
	defer pRegCloseKey.Call(key)
	if !on {
		pRegDeleteValueW.Call(key, uintptr(unsafe.Pointer(u16(runName))))
		return
	}
	v, _ := syscall.UTF16FromString(`"` + exe + `" --background`)
	pRegSetValueExW.Call(key, uintptr(unsafe.Pointer(u16(runName))), 0, regSZ, uintptr(unsafe.Pointer(&v[0])), uintptr(len(v)*2))
}
