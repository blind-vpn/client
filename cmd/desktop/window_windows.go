//go:build windows

package main

import (
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"github.com/jchv/go-webview2"
)

var windowMu sync.Mutex
var windowOpen bool

func openWindow(url string) bool {
	windowMu.Lock()
	if windowOpen {
		windowMu.Unlock()
		return true
	}
	windowOpen = true
	windowMu.Unlock()

	runtime.LockOSThread()
	defer runtime.UnlockOSThread()

	w := webview2.NewWithOptions(webview2.WebViewOptions{
		Debug:     false,
		AutoFocus: true,
		WindowOptions: webview2.WindowOptions{
			Title:  "Blind VPN",
			Width:  420,
			Height: 640,
			Center: true,
		},
	})
	if w == nil {
		windowMu.Lock()
		windowOpen = false
		windowMu.Unlock()
		return false
	}

	w.SetSize(420, 640, webview2.HintFixed)

	// Set window icon from embedded tray icon
	go setWindowIcon()

	w.Navigate(url)
	w.Run()
	w.Destroy()

	windowMu.Lock()
	windowOpen = false
	windowMu.Unlock()
	return true
}

func setWindowIcon() {
	time.Sleep(300 * time.Millisecond)

	// Find the webview window
	className, _ := syscall.UTF16PtrFromString("webview")
	hwnd, _, _ := user32.NewProc("FindWindowW").Call(uintptr(unsafe.Pointer(className)), 0)
	if hwnd == 0 {
		return
	}

	// Extract icon to temp
	data, err := trayIcons.ReadFile("tray_connected.ico")
	if err != nil {
		return
	}
	tmpPath := filepath.Join(os.TempDir(), "blindvpn-wicon.ico")
	os.WriteFile(tmpPath, data, 0600)
	icoPath, _ := syscall.UTF16PtrFromString(tmpPath)

	const (
		IMAGE_ICON      = 1
		LR_LOADFROMFILE = 0x0010
		WM_SETICON      = 0x0080
	)

	big, _, _ := user32.NewProc("LoadImageW").Call(0, uintptr(unsafe.Pointer(icoPath)), IMAGE_ICON, 32, 32, LR_LOADFROMFILE)
	small, _, _ := user32.NewProc("LoadImageW").Call(0, uintptr(unsafe.Pointer(icoPath)), IMAGE_ICON, 16, 16, LR_LOADFROMFILE)

	if big != 0 {
		user32.NewProc("SendMessageW").Call(hwnd, WM_SETICON, 1, big)
	}
	if small != 0 {
		user32.NewProc("SendMessageW").Call(hwnd, WM_SETICON, 0, small)
	}
}

func showAppWindow() {}
func hideAppWindow() {}
