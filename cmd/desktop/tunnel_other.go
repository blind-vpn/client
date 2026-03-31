//go:build !windows

package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

type unixTunnel struct {
	parent *Tunnel
}

func newTunnelImpl(t *Tunnel) tunnelImpl {
	return &unixTunnel{parent: t}
}

func (ut *unixTunnel) Connect(cfg TunnelConfig) error {
	cmd := exec.Command("sudo", "wg-quick", "up", ut.parent.configPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("wg-quick up: %w", err)
	}
	return nil
}

func (ut *unixTunnel) Disconnect() error {
	name := ut.parent.configPath
	if name == "" {
		name = ut.parent.ifaceName
	}
	cmd := exec.Command("sudo", "wg-quick", "down", name)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("wg-quick down: %s", strings.TrimSpace(string(out)))
	}
	return nil
}
