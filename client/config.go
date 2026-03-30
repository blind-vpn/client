package client

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

type CLIConfig struct {
	APIURL     string            `json:"api_url"`
	AccountID  string            `json:"account_id"`
	PrivateKey string            `json:"private_key"`
	PublicKey  string            `json:"public_key"`
	Active     *ActiveConnection `json:"active_connection,omitempty"`
}

type ActiveConnection struct {
	ServerID  string `json:"server_id"`
	Interface string `json:"interface"`
}

func ConfigPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".vpnservice", "config.json")
}

func WGConfigDir() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".vpnservice", "wg")
}

func LoadConfig() (*CLIConfig, error) {
	data, err := os.ReadFile(ConfigPath())
	if err != nil {
		if os.IsNotExist(err) {
			return &CLIConfig{}, nil
		}
		return nil, fmt.Errorf("read config: %w", err)
	}
	var cfg CLIConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parse config: %w", err)
	}
	return &cfg, nil
}

func SaveConfig(cfg *CLIConfig) error {
	dir := filepath.Dir(ConfigPath())
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("create config dir: %w", err)
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal config: %w", err)
	}
	return os.WriteFile(ConfigPath(), data, 0600)
}
