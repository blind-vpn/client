//go:build windows

package main

import (
	"fmt"
	"syscall"
	"unsafe"
)

// ensureSingleInstance creates a named mutex. Returns an error if already running.
func ensureSingleInstance() error {
	name, _ := syscall.UTF16PtrFromString("Global\\BlindVPN_SingleInstance")
	h, _, err := kernel32.NewProc("CreateMutexW").Call(0, 0, uintptr(unsafe.Pointer(name)))
	if h == 0 {
		return fmt.Errorf("create mutex: %v", err)
	}
	const ERROR_ALREADY_EXISTS = 183
	if err.(syscall.Errno) == ERROR_ALREADY_EXISTS {
		return fmt.Errorf("already running")
	}
	// Don't close the handle — keep it alive for the process lifetime
	return nil
}
