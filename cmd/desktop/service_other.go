//go:build !windows

package main

import "net/http"

const servicePort = "127.0.0.1:52780"

func isServiceMode() bool              { return false }
func runService(app *App, h http.Handler) {}
func ensureServiceInstalled() error    { return nil }
func serviceRunning() bool             { return false }
func installService() error            { return nil }
