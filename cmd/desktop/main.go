package main

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"runtime"
	"strings"
	"syscall"
)

//go:embed ui
var uiFS embed.FS

const defaultAPI = "https://blind-vpn.com"

func main() {
	app := NewApp(defaultAPI)

	mux := http.NewServeMux()

	// API routes — wrapped with CSRF protection
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

	// Serve embedded UI
	uiContent, _ := fs.Sub(uiFS, "ui")
	fileServer := http.FileServer(http.FS(uiContent))
	mux.Handle("/", fileServer)

	// Listen on random port
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to start: %v\n", err)
		os.Exit(1)
	}
	port := listener.Addr().(*net.TCPAddr).Port
	url := fmt.Sprintf("http://127.0.0.1:%d", port)

	go http.Serve(listener, protected)

	// Autoconnect on startup
	if app.settings.Autoconnect && app.settings.LastServer != nil && app.accountID != "" && app.privKey != "" {
		go app.autoConnect()
	}

	// Try native window (Windows WebView2), fall back to browser
	if !openWindow(url) {
		fmt.Printf("Blind VPN running at %s\n", url)
		openBrowser(url)

		sig := make(chan os.Signal, 1)
		signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
		<-sig
	}

	if app.tunnel != nil {
		app.tunnel.Disconnect()
	}
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
	KillSwitch  bool
	Padding     bool
	LastServer  *savedServer
}

// App holds all state
type App struct {
	apiURL    string
	accountID string
	privKey   string
	pubKey    string
	tunnel    *Tunnel
	configDir string
	settings  appSettings
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
		appData := os.Getenv("APPDATA")
		if appData == "" {
			appData = os.Getenv("USERPROFILE")
		}
		return filepath.Join(appData, "BlindVPN")
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
	KillSwitch     bool         `json:"kill_switch"`
	Padding        bool         `json:"padding"`
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
	a.settings.KillSwitch = s.KillSwitch
	a.settings.Padding = s.Padding
	a.settings.LastServer = s.LastServer

	// Decrypt private key (DPAPI on Windows, AES-GCM on others)
	if len(s.PrivKeyEnc) > 0 {
		dec, err := decryptCredential(s.PrivKeyEnc)
		if err == nil {
			a.privKey = string(dec)
		} else {
			// Migration: old unencrypted data — treat as plaintext, will be encrypted on next save
			a.privKey = string(s.PrivKeyEnc)
		}
	} else if s.PrivKey != "" {
		// Migration: old plaintext key — will be encrypted on next save
		a.privKey = s.PrivKey
	}
}

func (a *App) saveState() {
	s := appState{
		PubKey:      a.pubKey,
		AccountID:   a.accountID,
		Autostart:   a.settings.Autostart,
		Autoconnect: a.settings.Autoconnect,
		KillSwitch:  a.settings.KillSwitch,
		Padding:     a.settings.Padding,
		LastServer:  a.settings.LastServer,
	}

	// Encrypt private key if possible (DPAPI on Windows)
	if a.privKey != "" {
		enc, err := encryptCredential([]byte(a.privKey))
		if err == nil && len(enc) > 0 {
			s.PrivKeyEnc = enc
		} else {
			s.PrivKey = a.privKey // fallback to plaintext
		}
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
	resp, err := apiPost(a.apiURL+"/v1/keys", map[string]any{"pubkey": a.pubKey, "padding": a.settings.Padding}, a.accountID)
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

	tunnel, err := NewTunnel(a.configDir)
	if err != nil {
		jsonError(w, err.Error(), 500)
		return
	}

	err = tunnel.Connect(TunnelConfig{
		PrivateKey:      a.privKey,
		ServerPublicKey: req.ServerPubKey,
		ServerEndpoint:  req.ServerIP,
		ServerPort:      req.ServerPort,
		TunnelAddress:   strings.TrimSuffix(req.AllowedIP, "/32"),
		DNS:             req.ServerIP,
	})
	if err != nil {
		jsonError(w, err.Error(), 500)
		return
	}

	a.tunnel = tunnel

	// Save last server for autoconnect
	a.settings.LastServer = &savedServer{
		ID:        req.ServerID,
		IP:        req.ServerIP,
		PubKey:    req.ServerPubKey,
		Port:      req.ServerPort,
		AllowedIP: req.AllowedIP,
	}
	a.saveState()

	if a.settings.KillSwitch {
		enableKillSwitch(req.ServerIP)
	}

	jsonOK(w, map[string]any{"status": "connected", "server_ip": req.ServerIP})
}

func (a *App) handleDisconnect(w http.ResponseWriter, r *http.Request) {
	if a.tunnel != nil {
		a.tunnel.Disconnect()
		a.tunnel = nil
	}
	disableKillSwitch()
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
		"kill_switch": a.settings.KillSwitch,
		"padding":     a.settings.Padding,
	})
}

func (a *App) handleUpdateSettings(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Autostart   *bool `json:"autostart"`
		Autoconnect *bool `json:"autoconnect"`
		KillSwitch  *bool `json:"kill_switch"`
		Padding     *bool `json:"padding"`
	}
	json.NewDecoder(r.Body).Decode(&req)

	if req.Autostart != nil {
		a.settings.Autostart = *req.Autostart
		setAutostart(*req.Autostart)
	}
	if req.Autoconnect != nil {
		a.settings.Autoconnect = *req.Autoconnect
	}
	if req.KillSwitch != nil {
		a.settings.KillSwitch = *req.KillSwitch
		if *req.KillSwitch && a.tunnel != nil && a.tunnel.Connected {
			enableKillSwitch(a.tunnel.ServerIP)
		} else if !*req.KillSwitch {
			disableKillSwitch()
		}
	}
	if req.Padding != nil {
		a.settings.Padding = *req.Padding
	}

	a.saveState()
	jsonOK(w, map[string]any{"ok": true})
}

func (a *App) autoConnect() {
	srv := a.settings.LastServer

	tunnel, err := NewTunnel(a.configDir)
	if err != nil {
		return
	}

	err = tunnel.Connect(TunnelConfig{
		PrivateKey:      a.privKey,
		ServerPublicKey: srv.PubKey,
		ServerEndpoint:  srv.IP,
		ServerPort:      srv.Port,
		TunnelAddress:   strings.TrimSuffix(srv.AllowedIP, "/32"),
		DNS:             srv.IP,
	})
	if err != nil {
		return
	}

	a.tunnel = tunnel
	if a.settings.KillSwitch {
		enableKillSwitch(srv.IP)
	}
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
