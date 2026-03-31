package main

import (
	"os"
	"os/exec"
	"syscall"

	"golang.org/x/sys/windows/registry"
)

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
		// Always point to the installed location, not wherever we're running from
		appData := os.Getenv("APPDATA")
		if appData == "" {
			return
		}
		installed := appData + `\BlindVPN\blindvpn.exe`
		k.SetStringValue(regValue, installed)
	} else {
		k.DeleteValue(regValue)
	}
}

