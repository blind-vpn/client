//go:build windows

package main

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows/svc"
	"golang.org/x/sys/windows/svc/mgr"
)

const (
	serviceName = "BlindVPN"
	servicePort = "127.0.0.1:52780"
)

func isServiceMode() bool {
	is, _ := svc.IsWindowsService()
	return is
}

func runService(app *App, handler http.Handler) {
	svc.Run(serviceName, &vpnService{app: app, handler: handler})
}

type vpnService struct {
	app     *App
	handler http.Handler
}

func (s *vpnService) Execute(args []string, r <-chan svc.ChangeRequest, changes chan<- svc.Status) (bool, uint32) {
	changes <- svc.Status{State: svc.StartPending}

	// Extract wintun.dll
	extractWintun()

	listener, err := net.Listen("tcp", servicePort)
	if err != nil {
		log.Printf("service listen: %v", err)
		return true, 1
	}

	server := &http.Server{Handler: s.handler}
	go server.Serve(listener)

	// Autoconnect disabled for now — will be added back properly
	// if s.app.settings.Autoconnect && s.app.settings.LastServer != nil {
	// }

	changes <- svc.Status{State: svc.Running, Accepts: svc.AcceptStop | svc.AcceptShutdown}

	for c := range r {
		switch c.Cmd {
		case svc.Stop, svc.Shutdown:
			changes <- svc.Status{State: svc.StopPending}
			if s.app.tunnel != nil {
				s.app.tunnel.Disconnect()
			}
			server.Close()
			return false, 0
		}
	}
	return false, 0
}

func installService() error {
	exePath, err := os.Executable()
	if err != nil {
		return err
	}

	m, err := mgr.Connect()
	if err != nil {
		return fmt.Errorf("connect to SCM: %w", err)
	}
	defer m.Disconnect()

	s, err := m.OpenService(serviceName)
	if err == nil {
		s.Close()
		return nil
	}

	s, err = m.CreateService(serviceName, exePath, mgr.Config{
		DisplayName: "Blind VPN Tunnel Service",
		Description: "Manages WireGuard VPN tunnels for Blind VPN",
		StartType:   mgr.StartAutomatic,
	}, "/service")
	if err != nil {
		return fmt.Errorf("create service: %w", err)
	}
	defer s.Close()
	return s.Start()
}

func serviceRunning() bool {
	resp, err := http.Get("http://" + servicePort + "/api/status")
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode == 200
}

// elevateAndInstall launches this exe as admin with /install flag.
func elevateAndInstall() error {
	exePath, _ := os.Executable()

	verb, _ := syscall.UTF16PtrFromString("runas")
	file, _ := syscall.UTF16PtrFromString(exePath)
	params, _ := syscall.UTF16PtrFromString("/install")

	type SHELLEXECUTEINFO struct {
		cbSize       uint32
		fMask        uint32
		hwnd         uintptr
		lpVerb       *uint16
		lpFile       *uint16
		lpParameters *uint16
		lpDirectory  *uint16
		nShow        int32
		hInstApp     uintptr
		lpIDList     uintptr
		lpClass      *uint16
		hkeyClass    uintptr
		dwHotKey     uint32
		hIcon        uintptr
		hProcess     uintptr
	}

	sei := SHELLEXECUTEINFO{
		lpVerb:       verb,
		lpFile:       file,
		lpParameters: params,
		nShow:        1,
	}
	sei.cbSize = uint32(unsafe.Sizeof(sei))

	shell32 := syscall.NewLazyDLL("shell32.dll")
	ret, _, _ := shell32.NewProc("ShellExecuteExW").Call(uintptr(unsafe.Pointer(&sei)))
	if ret == 0 {
		return fmt.Errorf("UAC cancelled or failed")
	}

	// Wait for service
	for i := 0; i < 30; i++ {
		time.Sleep(500 * time.Millisecond)
		if serviceRunning() {
			return nil
		}
	}
	return fmt.Errorf("service did not start")
}

func ensureServiceInstalled() error {
	if serviceRunning() {
		return nil
	}

	// Try direct install (if already admin)
	if installService() == nil {
		for i := 0; i < 20; i++ {
			time.Sleep(500 * time.Millisecond)
			if serviceRunning() {
				return nil
			}
		}
	}

	// Need elevation
	return elevateAndInstall()
}
