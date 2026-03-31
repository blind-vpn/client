//go:build !windows

package main

func ensureSingleInstance() error {
	return nil
}
