//go:build !windows

package main

// Tray is a no-op on non-Windows platforms.
type Tray struct{}

func NewTray(onShow, onConnect, onDisconnect, onQuit func()) *Tray { return &Tray{} }
func (t *Tray) Run()                                                {}
func (t *Tray) SetConnected(connected bool)                         {}
func (t *Tray) Quit()                                               {}
