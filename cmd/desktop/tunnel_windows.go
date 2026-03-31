//go:build windows

package main

import (
	"embed"
	"encoding/base64"
	"fmt"
	"net/netip"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"golang.zx2c4.com/wireguard/conn"
	"golang.zx2c4.com/wireguard/device"
	"golang.zx2c4.com/wireguard/tun"
)

//go:embed wintun.dll
var wintunDLL embed.FS

var wintunOnce sync.Once

func extractWintun() {
	wintunOnce.Do(func() {
		exePath, err := os.Executable()
		if err != nil {
			return
		}
		dllPath := filepath.Join(filepath.Dir(exePath), "wintun.dll")
		if _, err := os.Stat(dllPath); err == nil {
			return
		}
		data, err := wintunDLL.ReadFile("wintun.dll")
		if err != nil {
			return
		}
		os.WriteFile(dllPath, data, 0644)
	})
}

type windowsTunnel struct {
	parent *Tunnel
	dev    *device.Device
	tdev   tun.Device
}

func newTunnelImpl(t *Tunnel) tunnelImpl {
	return &windowsTunnel{parent: t}
}

func (wt *windowsTunnel) Connect(cfg TunnelConfig) error {
	extractWintun()
	wt.parent.Progress("Creating tunnel...")

	tunDev, err := tun.CreateTUN("BlindVPN", 1420)
	if err != nil {
		if !isAdmin() {
			return fmt.Errorf("Run as Administrator to create the VPN tunnel")
		}
		return wt.fallbackConnect(cfg)
	}
	wt.tdev = tunDev

	privKey, err := base64.StdEncoding.DecodeString(cfg.PrivateKey)
	if err != nil {
		tunDev.Close()
		return fmt.Errorf("decode private key: %w", err)
	}
	pubKey, err := base64.StdEncoding.DecodeString(cfg.ServerPublicKey)
	if err != nil {
		tunDev.Close()
		return fmt.Errorf("decode server public key: %w", err)
	}

	dev := device.NewDevice(tunDev, conn.NewDefaultBind(), device.NewLogger(device.LogLevelSilent, ""))
	wt.dev = dev

	endpoint := fmt.Sprintf("%s:%d", cfg.ServerEndpoint, cfg.ServerPort)
	ipcConf := fmt.Sprintf(`private_key=%x
public_key=%x
endpoint=%s
allowed_ip=0.0.0.0/0
allowed_ip=::/0
persistent_keepalive_interval=25
`, privKey, pubKey, endpoint)

	if err := dev.IpcSet(ipcConf); err != nil {
		dev.Close()
		tunDev.Close()
		return fmt.Errorf("configure wireguard: %w", err)
	}

	wt.parent.Progress("Starting WireGuard...")
	if err := dev.Up(); err != nil {
		dev.Close()
		tunDev.Close()
		return fmt.Errorf("bring up wireguard: %w", err)
	}

	// Configure IP address and routing via netsh (uses interface name, not index)
	ifName, _ := tunDev.Name()

	runCmd("netsh", "interface", "ip", "set", "address", ifName, "static", cfg.TunnelAddress, "255.255.255.255")

	if cfg.DNS != "" {
		runCmd("netsh", "interface", "ip", "set", "dns", ifName, "static", cfg.DNS)
	}

	wt.parent.Progress("Configuring network...")
	// Get interface index for route commands
	ifIdx := getInterfaceIndex(ifName)

	// Routing: route VPN server via current default gateway, then default via tunnel
	gw := getDefaultGateway()
	if gw != "" {
		runCmd("route", "add", cfg.ServerEndpoint, "mask", "255.255.255.255", gw)
	}

	// Split default routes through the tunnel
	if ifIdx != "" {
		runCmd("route", "add", "0.0.0.0", "mask", "128.0.0.0", cfg.TunnelAddress, "metric", "5", "if", ifIdx)
		runCmd("route", "add", "128.0.0.0", "mask", "128.0.0.0", cfg.TunnelAddress, "metric", "5", "if", ifIdx)
	}

	wt.parent.Progress("Waiting for handshake...")
	// VERIFY: wait for WireGuard handshake.
	// Server may need a few seconds to sync the peer, so retry the handshake check.
	if err := wt.waitForHandshake(15 * time.Second); err != nil {
		wt.Disconnect()
		return fmt.Errorf("connection failed: no response from server")
	}

	return nil
}

// waitForHandshake polls the WireGuard device for a successful handshake.
func (wt *windowsTunnel) waitForHandshake(timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		ipcGet, err := wt.dev.IpcGet()
		if err == nil && strings.Contains(ipcGet, "last_handshake_time_sec=") {
			for _, line := range strings.Split(ipcGet, "\n") {
				if strings.HasPrefix(line, "last_handshake_time_sec=") {
					val := strings.TrimPrefix(line, "last_handshake_time_sec=")
					if val != "0" && val != "" {
						return nil // handshake completed
					}
				}
			}
		}
		time.Sleep(500 * time.Millisecond)
	}
	return fmt.Errorf("handshake timeout")
}

func (wt *windowsTunnel) Disconnect() error {
	if wt.dev != nil {
		wt.dev.Close()
		wt.dev = nil
	}
	if wt.tdev != nil {
		wt.tdev.Close()
		wt.tdev = nil
	}
	if wt.parent.ServerIP != "" {
		runCmd("route", "delete", wt.parent.ServerIP)
	}
	// Routes are cleaned up automatically when the TUN interface is destroyed
	return nil
}

func (wt *windowsTunnel) fallbackConnect(cfg TunnelConfig) error {
	paths := []string{
		"wireguard.exe",
		`C:\Program Files\WireGuard\wireguard.exe`,
	}
	for _, wg := range paths {
		cmd := exec.Command(wg, "/installtunnelservice", wt.parent.configPath)
		if err := cmd.Run(); err == nil {
			return nil
		}
	}
	return fmt.Errorf("WireGuard not available. Install from https://www.wireguard.com/install/ or run as Administrator.")
}

func runCmd(name string, args ...string) error {
	return exec.Command(name, args...).Run()
}

func isAdmin() bool {
	_, err := os.Open("\\\\.\\PHYSICALDRIVE0")
	return err == nil
}

func getInterfaceIndex(name string) string {
	out, err := exec.Command("netsh", "interface", "ip", "show", "interfaces").CombinedOutput()
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(out), "\n") {
		if strings.Contains(line, name) {
			fields := strings.Fields(strings.TrimSpace(line))
			if len(fields) >= 1 {
				return fields[0]
			}
		}
	}
	return ""
}

func getDefaultGateway() string {
	out, err := exec.Command("route", "print", "0.0.0.0").CombinedOutput()
	if err != nil {
		return ""
	}
	for _, line := range strings.Split(string(out), "\n") {
		fields := strings.Fields(strings.TrimSpace(line))
		if len(fields) >= 3 && fields[0] == "0.0.0.0" && fields[1] == "0.0.0.0" {
			if _, err := netip.ParseAddr(fields[2]); err == nil {
				return fields[2]
			}
		}
	}
	return ""
}
