package main

func openWindow(url string) bool {
	// WebView2 requires native Windows build tooling.
	// For now, fall back to browser on all builds.
	// To enable native window, build locally on Windows with:
	//   go build -tags webview ./cmd/desktop
	return false
}
