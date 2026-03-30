package main

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"

	"golang.org/x/sys/windows/registry"
)

// hiddenCmd creates an exec.Cmd that runs without showing a console window.
func hiddenCmd(name string, args ...string) *exec.Cmd {
	cmd := exec.Command(name, args...)
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	return cmd
}

const regKey = `Software\Microsoft\Windows\CurrentVersion\Run`
const regValue = "BlindVPN"

func setAutostart(enable bool) {
	k, err := registry.OpenKey(registry.CURRENT_USER, regKey, registry.SET_VALUE)
	if err != nil {
		return
	}
	defer k.Close()

	if enable {
		exe, err := os.Executable()
		if err != nil {
			return
		}
		k.SetStringValue(regValue, exe)
	} else {
		k.DeleteValue(regValue)
	}
}

const killSwitchRuleName = "BlindVPN-KillSwitch"

func enableKillSwitch(vpnServerIP string) {
	// Remove old rules first
	disableKillSwitch()

	// Block all outbound traffic
	hiddenCmd("netsh", "advfirewall", "firewall", "add", "rule",
		"name="+killSwitchRuleName+"-BlockAll",
		"dir=out", "action=block", "enable=yes",
		"profile=any", "localip=any", "remoteip=any",
	).Run()

	// Allow traffic to VPN server (WireGuard handshake)
	hiddenCmd("netsh", "advfirewall", "firewall", "add", "rule",
		"name="+killSwitchRuleName+"-AllowVPN",
		"dir=out", "action=allow", "enable=yes",
		"profile=any",
		fmt.Sprintf("remoteip=%s", vpnServerIP),
		"protocol=udp", "remoteport=51820",
	).Run()

	// Allow traffic on the WireGuard tunnel interface (10.66.0.0/16)
	hiddenCmd("netsh", "advfirewall", "firewall", "add", "rule",
		"name="+killSwitchRuleName+"-AllowTunnel",
		"dir=out", "action=allow", "enable=yes",
		"profile=any",
		"localip=10.66.0.0/16",
	).Run()

	// Allow localhost (needed for the app's local HTTP server)
	hiddenCmd("netsh", "advfirewall", "firewall", "add", "rule",
		"name="+killSwitchRuleName+"-AllowLocal",
		"dir=out", "action=allow", "enable=yes",
		"profile=any",
		"remoteip=127.0.0.0/8",
	).Run()

	// Allow DHCP
	hiddenCmd("netsh", "advfirewall", "firewall", "add", "rule",
		"name="+killSwitchRuleName+"-AllowDHCP",
		"dir=out", "action=allow", "enable=yes",
		"profile=any",
		"protocol=udp", "localport=68", "remoteport=67",
	).Run()
}

func disableKillSwitch() {
	// Remove all kill switch rules
	hiddenCmd("netsh", "advfirewall", "firewall", "delete", "rule",
		"name="+killSwitchRuleName+"-BlockAll",
	).Run()
	hiddenCmd("netsh", "advfirewall", "firewall", "delete", "rule",
		"name="+killSwitchRuleName+"-AllowVPN",
	).Run()
	hiddenCmd("netsh", "advfirewall", "firewall", "delete", "rule",
		"name="+killSwitchRuleName+"-AllowTunnel",
	).Run()
	hiddenCmd("netsh", "advfirewall", "firewall", "delete", "rule",
		"name="+killSwitchRuleName+"-AllowLocal",
	).Run()
	hiddenCmd("netsh", "advfirewall", "firewall", "delete", "rule",
		"name="+killSwitchRuleName+"-AllowDHCP",
	).Run()
}
