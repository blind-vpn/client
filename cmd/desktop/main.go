package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
)

//go:embed ui
var uiFS embed.FS

const defaultAPI = "https://blind-vpn.com"

func main() {
	// Handle /install flag
	if len(os.Args) > 1 && os.Args[1] == "/install" {
		if err := installService(); err != nil {
			fmt.Fprintf(os.Stderr, "install failed: %v\n", err)
			os.Exit(1)
		}
		os.Exit(0)
	}

	app := NewApp(defaultAPI)
	mux := http.NewServeMux()
	protected := csrfProtect(mux)

	mux.HandleFunc("/api/account/create", app.handleCreateAccount)
	mux.HandleFunc("/api/account/info", app.handleAccountInfo)
	mux.HandleFunc("/api/servers", app.handleServers)
	mux.HandleFunc("/api/keys/register", app.handleRegisterKey)
	mux.HandleFunc("/api/connect", app.handleConnect)
	mux.HandleFunc("/api/disconnect", app.handleDisconnect)
	mux.HandleFunc("/api/status", app.handleStatus)
	mux.HandleFunc("/api/settings", app.handleSettings)
	mux.HandleFunc("/api/settings/update", app.handleUpdateSettings)
	mux.HandleFunc("/api/events", app.handleEvents)

	mux.HandleFunc("/api/open", func(w http.ResponseWriter, r *http.Request) {
		u := r.URL.Query().Get("url")
		if u != "" && (strings.HasPrefix(u, "https://blind-vpn.com") || strings.HasPrefix(u, "https://www.blind-vpn.com")) {
			openBrowser(u)
		}
		jsonOK(w, map[string]any{"ok": true})
	})

	uiContent, _ := fs.Sub(uiFS, "ui")
	mux.Handle("/", http.FileServer(http.FS(uiContent)))

	// SERVICE MODE
	if isServiceMode() {
		runService(app, protected)
		return
	}

	// GUI MODE
	if err := ensureSingleInstance(); err != nil {
		os.Exit(0)
	}

	if err := ensureServiceInstalled(); err != nil {
		fmt.Fprintf(os.Stderr, "Service setup failed: %v\n", err)
		listener, err := net.Listen("tcp", servicePort)
		if err != nil {
			os.Exit(1)
		}
		go http.Serve(listener, protected)
	}

	url := "http://" + servicePort

	// Tray icon
	var tray *Tray
	tray = NewTray(
		func() { go openWindow(url) },       // Show
		func() { /* TODO: quick connect */ }, // Connect
		func() {                              // Disconnect
			http.Post("http://"+servicePort+"/api/disconnect", "", nil)
			tray.SetConnected(false)
		},
		func() { // Quit
			http.Post("http://"+servicePort+"/api/disconnect", "", nil)
			tray.Quit()
		},
	)

	// Tray status updates
	mux.HandleFunc("/api/tray/connected", func(w http.ResponseWriter, r *http.Request) {
		tray.SetConnected(true)
		jsonOK(w, map[string]any{"ok": true})
	})
	mux.HandleFunc("/api/tray/disconnected", func(w http.ResponseWriter, r *http.Request) {
		tray.SetConnected(false)
		jsonOK(w, map[string]any{"ok": true})
	})

	// Subscribe to service events and update tray icon
	go func() {
		for {
			resp, err := http.Get("http://" + servicePort + "/api/events")
			if err != nil {
				time.Sleep(2 * time.Second)
				continue
			}
			buf := make([]byte, 256)
			for {
				n, err := resp.Body.Read(buf)
				if err != nil {
					break
				}
				msg := strings.TrimSpace(string(buf[:n]))
				for _, line := range strings.Split(msg, "\n") {
					line = strings.TrimPrefix(line, "data: ")
					switch line {
					case "connected":
						tray.SetConnected(true)
					case "disconnected":
						tray.SetConnected(false)
					}
				}
			}
			resp.Body.Close()
			time.Sleep(1 * time.Second)
		}
	}()

	// Open window
	go func() {
		if !openWindow(url) {
			openBrowser(url)
		}
	}()

	tray.Run()
}

func openBrowser(url string) {
	var args []string
	switch runtime.GOOS {
	case "windows":
		args = []string{"rundll32", "url.dll,FileProtocolHandler", url}
	case "darwin":
		args = []string{"open", url}
	default:
		args = []string{"xdg-open", url}
	}
	cmd := newCommand(args[0], args[1:]...)
	cmd.Start()
}

type appSettings struct {
	Autostart   bool
	Autoconnect bool


	LastServer  *savedServer
}

// App holds all state
type App struct {
	apiURL      string
	accountID   string
	privKey     string
	pubKey      string
	tunnel      *Tunnel
	configDir   string
	settings    appSettings
	eventSubs   []chan string
	eventSubsMu sync.Mutex
}

// emit sends an event to all SSE subscribers.
func (a *App) emit(event string) {
	a.eventSubsMu.Lock()
	defer a.eventSubsMu.Unlock()
	for _, ch := range a.eventSubs {
		select {
		case ch <- event:
		default:
		}
	}
}

func (a *App) subscribe() chan string {
	ch := make(chan string, 8)
	a.eventSubsMu.Lock()
	a.eventSubs = append(a.eventSubs, ch)
	a.eventSubsMu.Unlock()
	return ch
}

func (a *App) unsubscribe(ch chan string) {
	a.eventSubsMu.Lock()
	defer a.eventSubsMu.Unlock()
	for i, c := range a.eventSubs {
		if c == ch {
			a.eventSubs = append(a.eventSubs[:i], a.eventSubs[i+1:]...)
			break
		}
	}
}

func (a *App) handleEvents(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return
	}

	ch := a.subscribe()
	defer a.unsubscribe(ch)

	// Send current state immediately
	if a.tunnel != nil && a.tunnel.Connected {
		fmt.Fprintf(w, "data: connected\n\n")
	} else {
		fmt.Fprintf(w, "data: disconnected\n\n")
	}
	flusher.Flush()

	for {
		select {
		case event := <-ch:
			fmt.Fprintf(w, "data: %s\n\n", event)
			flusher.Flush()
		case <-r.Context().Done():
			return
		}
	}
}

func NewApp(apiURL string) *App {
	configDir := getConfigDir()
	os.MkdirAll(configDir, 0700)

	app := &App{
		apiURL:    apiURL,
		configDir: configDir,
	}
	app.loadState()
	return app
}

func getConfigDir() string {
	if runtime.GOOS == "windows" {
		// Use ProgramData so both the service (SYSTEM) and the GUI (user) can access it
		programData := os.Getenv("ProgramData")
		if programData == "" {
			programData = `C:\ProgramData`
		}
		return filepath.Join(programData, "BlindVPN")
	}
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".blindvpn")
}

type appState struct {
	AccountID      string       `json:"account_id"`
	PrivKeyEnc     []byte       `json:"private_key_enc,omitempty"` // DPAPI-encrypted on Windows
	PrivKey        string       `json:"private_key,omitempty"`     // plaintext fallback (non-Windows, migration)
	PubKey         string       `json:"public_key"`
	Autostart      bool         `json:"autostart"`
	Autoconnect    bool         `json:"autoconnect"`


	LastServer     *savedServer `json:"last_server,omitempty"`
}

type savedServer struct {
	ID        string `json:"id"`
	IP        string `json:"ip"`
	PubKey    string `json:"pubkey"`
	Port      int    `json:"port"`
	AllowedIP string `json:"allowed_ip"`
	City      string `json:"city"`
	Country   string `json:"country"`
}

func (a *App) loadState() {
	data, err := os.ReadFile(filepath.Join(a.configDir, "state.json"))
	if err != nil {
		return
	}
	var s appState
	json.Unmarshal(data, &s)
	a.accountID = s.AccountID
	a.pubKey = s.PubKey
	a.settings.Autostart = s.Autostart
	a.settings.Autoconnect = s.Autoconnect


	a.settings.LastServer = s.LastServer

	// Load private key — try encrypted first, fall back to plaintext
	if len(s.PrivKeyEnc) > 0 {
		dec, err := decryptCredential(s.PrivKeyEnc)
		if err == nil {
			a.privKey = string(dec)
		}
	}
	if a.privKey == "" && s.PrivKey != "" {
		a.privKey = s.PrivKey
	}
}

func (a *App) saveState() {
	s := appState{
		PubKey:      a.pubKey,
		AccountID:   a.accountID,
		Autostart:   a.settings.Autostart,
		Autoconnect: a.settings.Autoconnect,


		LastServer:  a.settings.LastServer,
	}

	// Always save plaintext key (service runs as SYSTEM, DPAPI won't work cross-user).
	// The state file is in ProgramData with restricted permissions.
	if a.privKey != "" {
		s.PrivKey = a.privKey
	}

	data, _ := json.Marshal(s)
	os.WriteFile(filepath.Join(a.configDir, "state.json"), data, 0600)
}

// Handlers

func (a *App) handleCreateAccount(w http.ResponseWriter, r *http.Request) {
	resp, err := apiPost(a.apiURL+"/v1/accounts", nil, "")
	if err != nil {
		jsonError(w, err.Error(), 500)
		return
	}

	a.accountID = resp["account_id"].(string)

	priv, pub, err := generateKeypair()
	if err != nil {
		jsonError(w, "keypair generation failed: "+err.Error(), 500)
		return
	}
	a.privKey = priv
	a.pubKey = pub
	a.saveState()

	jsonOK(w, map[string]any{
		"account_id": a.accountID,
		"expires_at": resp["expires_at"],
		"public_key": a.pubKey,
	})
}

func (a *App) handleAccountInfo(w http.ResponseWriter, r *http.Request) {
	// Login: if ?id= is provided, switch to that account
	if id := r.URL.Query().Get("id"); id != "" && len(id) == 16 {
		// Verify the account exists on the server
		resp, err := apiGet(a.apiURL+"/v1/accounts/"+id, id)
		if err != nil {
			jsonError(w, err.Error(), 500)
			return
		}
		a.accountID = id
		// Generate keypair if we don't have one
		if a.privKey == "" {
			priv, pub, err := generateKeypair()
			if err != nil {
				jsonError(w, "keypair generation failed: "+err.Error(), 500)
				return
			}
			a.privKey = priv
			a.pubKey = pub
		}
		a.saveState()
		resp["public_key"] = a.pubKey
		jsonOK(w, resp)
		return
	}

	if a.accountID == "" {
		jsonError(w, "not logged in", 401)
		return
	}

	resp, err := apiGet(a.apiURL+"/v1/accounts/"+a.accountID, a.accountID)
	if err != nil {
		jsonError(w, err.Error(), 500)
		return
	}
	resp["public_key"] = a.pubKey
	jsonOK(w, resp)
}

func (a *App) handleServers(w http.ResponseWriter, r *http.Request) {
	if a.accountID == "" {
		jsonError(w, "not logged in", 401)
		return
	}
	resp, err := apiGet(a.apiURL+"/v1/servers", a.accountID)
	if err != nil {
		jsonError(w, err.Error(), 500)
		return
	}
	jsonOK(w, resp)
}

func (a *App) handleRegisterKey(w http.ResponseWriter, r *http.Request) {
	if a.accountID == "" || a.pubKey == "" {
		jsonError(w, "not logged in or no keypair", 401)
		return
	}
	resp, err := apiPost(a.apiURL+"/v1/keys", map[string]any{"pubkey": a.pubKey}, a.accountID)
	if err != nil {
		if strings.Contains(err.Error(), "duplicate") || strings.Contains(err.Error(), "unique") {
			keys, err2 := apiGet(a.apiURL+"/v1/keys", a.accountID)
			if err2 != nil {
				jsonError(w, err2.Error(), 500)
				return
			}
			jsonOK(w, keys)
			return
		}
		jsonError(w, err.Error(), 500)
		return
	}
	jsonOK(w, resp)
}

// doConnect is the single code path for all connections (UI and autoconnect).
func (a *App) doConnect(srv *savedServer) error {
	tunnel, err := NewTunnel(a.configDir)
	if err != nil {
		return err
	}
	tunnel.onProgress = func(msg string) { a.emit("progress:" + msg) }

	err = tunnel.Connect(TunnelConfig{
		PrivateKey:      a.privKey,
		ServerPublicKey: srv.PubKey,
		ServerEndpoint:  srv.IP,
		ServerPort:      srv.Port,
		TunnelAddress:   strings.TrimSuffix(srv.AllowedIP, "/32"),
		DNS:             srv.IP,
	})
	if err != nil {
		return err
	}

	a.tunnel = tunnel
	a.settings.LastServer = srv
	a.saveState()
	a.emit("connected")
	return nil
}

func (a *App) handleConnect(w http.ResponseWriter, r *http.Request) {
	if a.accountID == "" || a.privKey == "" {
		jsonError(w, "not logged in", 401)
		return
	}

	var req struct {
		ServerID     string `json:"server_id"`
		ServerIP     string `json:"server_ip"`
		ServerPubKey string `json:"server_pubkey"`
		ServerPort   int    `json:"server_port"`
		AllowedIP    string `json:"allowed_ip"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	srv := &savedServer{
		ID:        req.ServerID,
		IP:        req.ServerIP,
		PubKey:    req.ServerPubKey,
		Port:      req.ServerPort,
		AllowedIP: req.AllowedIP,
	}

	if err := a.doConnect(srv); err != nil {
		jsonError(w, err.Error(), 500)
		return
	}

	jsonOK(w, map[string]any{"status": "connected", "server_ip": req.ServerIP})
}

func (a *App) handleDisconnectDirect() {
	if a.tunnel != nil {
		a.tunnel.Disconnect()
		a.tunnel = nil
	}
	a.emit("disconnected")
}

func (a *App) handleDisconnect(w http.ResponseWriter, r *http.Request) {
	if a.tunnel != nil {
		a.tunnel.Disconnect()
		a.tunnel = nil
	}
	a.emit("disconnected")
	jsonOK(w, map[string]any{"status": "disconnected"})
}

func (a *App) handleStatus(w http.ResponseWriter, r *http.Request) {
	connected := a.tunnel != nil && a.tunnel.Connected
	resp := map[string]any{
		"connected":  connected,
		"account_id": a.accountID,
		"logged_in":  a.accountID != "",
	}
	if connected {
		resp["server_ip"] = a.tunnel.ServerIP
	}
	jsonOK(w, resp)
}

func (a *App) handleSettings(w http.ResponseWriter, r *http.Request) {
	jsonOK(w, map[string]any{
		"autostart":   a.settings.Autostart,
		"autoconnect": a.settings.Autoconnect,


	})
}

func (a *App) handleUpdateSettings(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Autostart   *bool `json:"autostart"`
		Autoconnect *bool `json:"autoconnect"`


	}
	json.NewDecoder(r.Body).Decode(&req)

	if req.Autostart != nil {
		a.settings.Autostart = *req.Autostart
		setAutostart(*req.Autostart)
	}
	if req.Autoconnect != nil {
		a.settings.Autoconnect = *req.Autoconnect
	}


	a.saveState()
	jsonOK(w, map[string]any{"ok": true})
}


func jsonOK(w http.ResponseWriter, data any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

func jsonError(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}

// csrfProtect blocks cross-origin requests to the local API.
// Only requests from the same origin (127.0.0.1) or non-browser clients are allowed.
func csrfProtect(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") {
			// Check Sec-Fetch-Site (modern browsers set this automatically)
			if sfs := r.Header.Get("Sec-Fetch-Site"); sfs != "" && sfs != "same-origin" && sfs != "none" {
				http.Error(w, "cross-origin request blocked", http.StatusForbidden)
				return
			}
			// Check Origin header
			if origin := r.Header.Get("Origin"); origin != "" &&
				!strings.HasPrefix(origin, "http://127.0.0.1:") &&
				!strings.HasPrefix(origin, "http://localhost:") {
				http.Error(w, "cross-origin request blocked", http.StatusForbidden)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}
