//go:build !windows

package main

func openWindow(url string) bool {
	return false
}
