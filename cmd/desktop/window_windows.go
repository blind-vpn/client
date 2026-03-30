//go:build windows

package main

import "github.com/jchv/go-webview2"

func openWindow(url string) bool {
	w := webview2.NewWithOptions(webview2.WebViewOptions{
		Debug:     false,
		AutoFocus: true,
		WindowOptions: webview2.WindowOptions{
			Title:  "Blind VPN",
			Width:  420,
			Height: 640,
			Center: true,
		},
	})
	if w == nil {
		return false
	}
	defer w.Destroy()
	w.SetSize(420, 640, webview2.HintFixed)
	w.Navigate(url)
	w.Run()
	return true
}
