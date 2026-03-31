//go:build !windows

package main

func setAutostart(enable bool) {
	// TODO: Linux: ~/.config/autostart/blindvpn.desktop
	// TODO: macOS: ~/Library/LaunchAgents/com.blindvpn.plist
}
