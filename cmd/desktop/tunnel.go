package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

type TunnelConfig struct {
	PrivateKey      string
	ServerPublicKey string
	ServerEndpoint  string
	ServerPort      int
	TunnelAddress   string
	DNS             string
}

type Tunnel struct {
	configDir  string
	configPath string
	ifaceName  string
	ServerIP   string
	Connected  bool
}

func NewTunnel(configDir string) (*Tunnel, error) {
	wgDir := filepath.Join(configDir, "wg")
	os.MkdirAll(wgDir, 0700)
	return &Tunnel{
		configDir: wgDir,
		ifaceName: "blindvpn",
	}, nil
}

func (t *Tunnel) Connect(cfg TunnelConfig) error {
	// Write WireGuard config
	conf := fmt.Sprintf(`[Interface]
PrivateKey = %s
Address = %s/32
DNS = %s

[Peer]
PublicKey = %s
AllowedIPs = 0.0.0.0/0, ::/0
Endpoint = %s:%d
PersistentKeepalive = 25
`, cfg.PrivateKey, cfg.TunnelAddress, cfg.DNS,
		cfg.ServerPublicKey, cfg.ServerEndpoint, cfg.ServerPort)

	t.configPath = filepath.Join(t.configDir, t.ifaceName+".conf")
	if err := os.WriteFile(t.configPath, []byte(conf), 0600); err != nil {
		return fmt.Errorf("write config: %w", err)
	}

	t.ServerIP = cfg.ServerEndpoint

	if runtime.GOOS == "windows" {
		return t.connectWindows()
	}
	return t.connectUnix()
}

func (t *Tunnel) connectWindows() error {
	// Try wireguard.exe (installed WireGuard)
	paths := []string{
		"wireguard.exe",
		`C:\Program Files\WireGuard\wireguard.exe`,
	}

	for _, wg := range paths {
		cmd := exec.Command(wg, "/installtunnelservice", t.configPath)
		if err := cmd.Run(); err == nil {
			t.Connected = true
			return nil
		}
	}

	// Try wg-quick via wsl as last resort
	cmd := exec.Command("wsl", "wg-quick", "up", t.configPath)
	if err := cmd.Run(); err == nil {
		t.Connected = true
		return nil
	}

	return fmt.Errorf(
		"WireGuard not found. Install it from:\nhttps://www.wireguard.com/install/\n\n" +
			"Or import the config file manually:\n" + t.configPath)
}

func (t *Tunnel) connectUnix() error {
	cmd := exec.Command("sudo", "wg-quick", "up", t.configPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("wg-quick up: %w", err)
	}
	t.Connected = true
	return nil
}

func (t *Tunnel) Disconnect() error {
	if !t.Connected {
		return nil
	}

	var err error
	if runtime.GOOS == "windows" {
		err = t.disconnectWindows()
	} else {
		err = t.disconnectUnix()
	}

	t.Connected = false
	return err
}

func (t *Tunnel) disconnectWindows() error {
	paths := []string{
		"wireguard.exe",
		`C:\Program Files\WireGuard\wireguard.exe`,
	}
	for _, wg := range paths {
		cmd := exec.Command(wg, "/uninstalltunnelservice", t.ifaceName)
		if err := cmd.Run(); err == nil {
			return nil
		}
	}
	return fmt.Errorf("could not disconnect tunnel")
}

func (t *Tunnel) disconnectUnix() error {
	name := t.configPath
	if name == "" {
		name = t.ifaceName
	}
	// Try config path first, then interface name
	cmd := exec.Command("sudo", "wg-quick", "down", name)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("wg-quick down: %s", strings.TrimSpace(string(out)))
	}
	return nil
}
