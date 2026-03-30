//go:build !windows

package main

func setAutostart(enable bool) {
	// TODO: Linux: ~/.config/autostart/blindvpn.desktop
	// TODO: macOS: ~/Library/LaunchAgents/com.blindvpn.plist
}

func enableKillSwitch(vpnServerIP string) {
	// TODO: Linux: iptables rules
	// TODO: macOS: pf rules
}

func disableKillSwitch() {
	// TODO: remove platform-specific firewall rules
}
