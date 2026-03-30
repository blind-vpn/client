package client

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
)

const defaultInterface = "wg-vpn"

func Connect(iface, configPath string) error {
	if iface == "" {
		iface = defaultInterface
	}
	cmd := exec.Command("wg-quick", "up", configPath)
	cmd.Env = append(os.Environ(), "WG_QUICK_USERSPACE_IMPLEMENTATION=")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("wg-quick up: %w", err)
	}
	return nil
}

func Disconnect(iface string) error {
	if iface == "" {
		iface = defaultInterface
	}
	cmd := exec.Command("wg-quick", "down", iface)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("wg-quick down: %w", err)
	}
	return nil
}

type ConnectionStatus struct {
	Interface string
	PublicKey string
	Endpoint  string
	Transfer  string
	Connected bool
}

func Status(iface string) (*ConnectionStatus, error) {
	if iface == "" {
		iface = defaultInterface
	}
	cmd := exec.Command("wg", "show", iface)
	out, err := cmd.Output()
	if err != nil {
		return &ConnectionStatus{Interface: iface, Connected: false}, nil
	}

	status := &ConnectionStatus{
		Interface: iface,
		Connected: true,
	}

	for _, line := range strings.Split(string(out), "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "public key:") {
			status.PublicKey = strings.TrimSpace(strings.TrimPrefix(line, "public key:"))
		}
		if strings.HasPrefix(line, "endpoint:") {
			status.Endpoint = strings.TrimSpace(strings.TrimPrefix(line, "endpoint:"))
		}
		if strings.HasPrefix(line, "transfer:") {
			status.Transfer = strings.TrimSpace(strings.TrimPrefix(line, "transfer:"))
		}
	}

	return status, nil
}
