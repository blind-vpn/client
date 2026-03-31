package main

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
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
	impl       tunnelImpl
	onProgress func(string)
}

func (t *Tunnel) Progress(msg string) {
	if t.onProgress != nil {
		t.onProgress(msg)
	}
}

// tunnelImpl is implemented per-platform.
type tunnelImpl interface {
	Connect(cfg TunnelConfig) error
	Disconnect() error
}

func NewTunnel(configDir string) (*Tunnel, error) {
	wgDir := filepath.Join(configDir, "wg")
	os.MkdirAll(wgDir, 0700)
	t := &Tunnel{
		configDir: wgDir,
		ifaceName: "blindvpn",
	}
	t.impl = newTunnelImpl(t)
	return t, nil
}

func (t *Tunnel) Connect(cfg TunnelConfig) error {
	t.ServerIP = cfg.ServerEndpoint
	t.configPath = filepath.Join(t.configDir, t.ifaceName+".conf")

	// Always write config for manual fallback
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
	os.WriteFile(t.configPath, []byte(conf), 0600)

	if err := t.impl.Connect(cfg); err != nil {
		return err
	}
	t.Connected = true
	return nil
}

func (t *Tunnel) Disconnect() error {
	if !t.Connected {
		return nil
	}
	err := t.impl.Disconnect()
	t.Connected = false
	return err
}

// writeConfig writes a WireGuard config file (for fallback / manual use).
func writeConfig(path string, cfg TunnelConfig) error {
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
	return os.WriteFile(path, []byte(conf), 0600)
}

func platformName() string {
	return runtime.GOOS
}
