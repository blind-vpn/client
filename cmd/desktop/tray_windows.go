//go:build windows

package main

import (
	"embed"
	"os"
	"path/filepath"
	"sync"
	"syscall"
	"unsafe"
)

//go:embed tray_connected.ico tray_disconnected.ico
var trayIcons embed.FS

var (
	shell32              = syscall.NewLazyDLL("shell32.dll")
	user32               = syscall.NewLazyDLL("user32.dll")
	kernel32             = syscall.NewLazyDLL("kernel32.dll")
	pShellNotifyIcon     = shell32.NewProc("Shell_NotifyIconW")
	pLoadImage           = user32.NewProc("LoadImageW")
	pCreateWindowEx      = user32.NewProc("CreateWindowExW")
	pDefWindowProc       = user32.NewProc("DefWindowProcW")
	pRegisterClassEx     = user32.NewProc("RegisterClassExW")
	pGetMessage          = user32.NewProc("GetMessageW")
	pTranslateMessage    = user32.NewProc("TranslateMessage")
	pDispatchMessage     = user32.NewProc("DispatchMessageW")
	pPostQuitMessage     = user32.NewProc("PostQuitMessage")
	pCreatePopupMenu     = user32.NewProc("CreatePopupMenu")
	pAppendMenu          = user32.NewProc("AppendMenuW")
	pTrackPopupMenu      = user32.NewProc("TrackPopupMenu")
	pDestroyMenu         = user32.NewProc("DestroyMenu")
	pSetForegroundWindow = user32.NewProc("SetForegroundWindow")
	pGetCursorPos        = user32.NewProc("GetCursorPos")
	pGetModuleHandle     = kernel32.NewProc("GetModuleHandleW")
)

const (
	NIM_ADD    = 0
	NIM_MODIFY = 1
	NIM_DELETE = 2
	NIF_ICON    = 2
	NIF_TIP     = 4
	NIF_MESSAGE = 1
	WM_USER     = 0x0400
	WM_TRAYICON = WM_USER + 1
	WM_COMMAND  = 0x0111
	WM_RBUTTONUP = 0x0205
	WM_LBUTTONDBLCLK = 0x0203
	IMAGE_ICON  = 1
	LR_LOADFROMFILE = 0x0010
	LR_DEFAULTSIZE  = 0x0040
	MF_STRING  = 0
	TPM_BOTTOMALIGN = 0x0020
	TPM_LEFTALIGN   = 0
	IDM_SHOW       = 1
	IDM_CONNECT    = 2
	IDM_DISCONNECT = 3
	IDM_QUIT       = 4
)

type NOTIFYICONDATA struct {
	CbSize           uint32
	HWnd             uintptr
	UID              uint32
	UFlags           uint32
	UCallbackMessage uint32
	HIcon            uintptr
	SzTip            [128]uint16
}

type WNDCLASSEX struct {
	CbSize        uint32
	Style         uint32
	LpfnWndProc   uintptr
	CbClsExtra    int32
	CbWndExtra    int32
	HInstance     uintptr
	HIcon         uintptr
	HCursor       uintptr
	HbrBackground uintptr
	LpszMenuName  uintptr
	LpszClassName uintptr
	HIconSm       uintptr
}

type MSG struct {
	HWnd    uintptr
	Message uint32
	WParam  uintptr
	LParam  uintptr
	Time    uint32
	Pt      struct{ X, Y int32 }
}

type POINT struct {
	X, Y int32
}

type Tray struct {
	hwnd          uintptr
	nid           NOTIFYICONDATA
	connectedIcon uintptr
	disconnIcon   uintptr
	connected     bool
	onShow        func()
	onConnect     func()
	onDisconnect  func()
	onQuit        func()
	mu            sync.Mutex
}

var globalTray *Tray

func NewTray(onShow, onConnect, onDisconnect, onQuit func()) *Tray {
	t := &Tray{
		onShow:       onShow,
		onConnect:    onConnect,
		onDisconnect: onDisconnect,
		onQuit:       onQuit,
	}
	globalTray = t
	return t
}

func (t *Tray) Run() {
	// Extract icon files to temp
	tmpDir, _ := os.MkdirTemp("", "blindvpn-tray")
	connPath := filepath.Join(tmpDir, "connected.ico")
	discPath := filepath.Join(tmpDir, "disconnected.ico")

	connData, _ := trayIcons.ReadFile("tray_connected.ico")
	discData, _ := trayIcons.ReadFile("tray_disconnected.ico")
	os.WriteFile(connPath, connData, 0600)
	os.WriteFile(discPath, discData, 0600)

	t.connectedIcon = loadIconFile(connPath)
	t.disconnIcon = loadIconFile(discPath)

	hInst, _, _ := pGetModuleHandle.Call(0)

	className, _ := syscall.UTF16PtrFromString("BlindVPNTray")
	wc := WNDCLASSEX{
		LpfnWndProc:   syscall.NewCallback(trayWndProc),
		HInstance:     hInst,
		LpszClassName: uintptr(unsafe.Pointer(className)),
	}
	wc.CbSize = uint32(unsafe.Sizeof(wc))
	pRegisterClassEx.Call(uintptr(unsafe.Pointer(&wc)))

	t.hwnd, _, _ = pCreateWindowEx.Call(
		0, uintptr(unsafe.Pointer(className)), 0, 0,
		0, 0, 0, 0, 0, 0, hInst, 0,
	)

	t.nid = NOTIFYICONDATA{
		HWnd:             t.hwnd,
		UID:              1,
		UFlags:           NIF_ICON | NIF_TIP | NIF_MESSAGE,
		UCallbackMessage: WM_TRAYICON,
		HIcon:            t.disconnIcon,
	}
	t.nid.CbSize = uint32(unsafe.Sizeof(t.nid))
	copy(t.nid.SzTip[:], utf16("Blind VPN - Disconnected"))
	pShellNotifyIcon.Call(NIM_ADD, uintptr(unsafe.Pointer(&t.nid)))

	var msg MSG
	for {
		ret, _, _ := pGetMessage.Call(uintptr(unsafe.Pointer(&msg)), 0, 0, 0)
		if ret == 0 {
			break
		}
		pTranslateMessage.Call(uintptr(unsafe.Pointer(&msg)))
		pDispatchMessage.Call(uintptr(unsafe.Pointer(&msg)))
	}

	pShellNotifyIcon.Call(NIM_DELETE, uintptr(unsafe.Pointer(&t.nid)))
	os.RemoveAll(tmpDir)
}

func (t *Tray) SetConnected(connected bool) {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.connected = connected
	if connected {
		t.nid.HIcon = t.connectedIcon
		copy(t.nid.SzTip[:], utf16("Blind VPN - Connected"))
	} else {
		t.nid.HIcon = t.disconnIcon
		copy(t.nid.SzTip[:], utf16("Blind VPN - Disconnected"))
	}
	pShellNotifyIcon.Call(NIM_MODIFY, uintptr(unsafe.Pointer(&t.nid)))
}

func (t *Tray) Quit() {
	pPostQuitMessage.Call(0)
}

func trayWndProc(hwnd, msg, wParam, lParam uintptr) uintptr {
	t := globalTray
	if t == nil {
		ret, _, _ := pDefWindowProc.Call(hwnd, msg, wParam, lParam)
		return ret
	}

	switch msg {
	case WM_TRAYICON:
		switch lParam {
		case WM_RBUTTONUP:
			showTrayMenu(t)
		case WM_LBUTTONDBLCLK:
			if t.onShow != nil {
				t.onShow()
			}
		}
	case WM_COMMAND:
		switch wParam {
		case IDM_SHOW:
			if t.onShow != nil {
				t.onShow()
			}
		case IDM_CONNECT:
			if t.onConnect != nil {
				t.onConnect()
			}
		case IDM_DISCONNECT:
			if t.onDisconnect != nil {
				t.onDisconnect()
			}
		case IDM_QUIT:
			if t.onQuit != nil {
				t.onQuit()
			}
		}
	}

	ret, _, _ := pDefWindowProc.Call(hwnd, msg, wParam, lParam)
	return ret
}

func showTrayMenu(t *Tray) {
	menu, _, _ := pCreatePopupMenu.Call()

	show, _ := syscall.UTF16PtrFromString("Show")
	pAppendMenu.Call(menu, MF_STRING, IDM_SHOW, uintptr(unsafe.Pointer(show)))

	t.mu.Lock()
	connected := t.connected
	t.mu.Unlock()

	if connected {
		disc, _ := syscall.UTF16PtrFromString("Disconnect")
		pAppendMenu.Call(menu, MF_STRING, IDM_DISCONNECT, uintptr(unsafe.Pointer(disc)))
	} else {
		conn, _ := syscall.UTF16PtrFromString("Connect")
		pAppendMenu.Call(menu, MF_STRING, IDM_CONNECT, uintptr(unsafe.Pointer(conn)))
	}

	quit, _ := syscall.UTF16PtrFromString("Quit")
	pAppendMenu.Call(menu, MF_STRING, IDM_QUIT, uintptr(unsafe.Pointer(quit)))

	var pt POINT
	pGetCursorPos.Call(uintptr(unsafe.Pointer(&pt)))
	pSetForegroundWindow.Call(t.hwnd)
	pTrackPopupMenu.Call(menu, TPM_BOTTOMALIGN|TPM_LEFTALIGN, uintptr(pt.X), uintptr(pt.Y), 0, t.hwnd, 0)
	pDestroyMenu.Call(menu)
}

func loadIconFile(path string) uintptr {
	p, _ := syscall.UTF16PtrFromString(path)
	h, _, _ := pLoadImage.Call(0, uintptr(unsafe.Pointer(p)), IMAGE_ICON, 0, 0, LR_LOADFROMFILE|LR_DEFAULTSIZE)
	return h
}

func utf16(s string) []uint16 {
	r, _ := syscall.UTF16FromString(s)
	return r
}
