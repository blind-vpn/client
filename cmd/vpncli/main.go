package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"
	"time"

	"vpnservice/client"

	"github.com/spf13/cobra"
)

var apiURL string

func main() {
	root := &cobra.Command{
		Use:   "vpncli",
		Short: "VPN service CLI client",
	}
	root.PersistentFlags().StringVar(&apiURL, "api-url", "http://localhost:8080", "API server URL")

	// ── Account commands ──
	accountCmd := &cobra.Command{Use: "account", Short: "Manage account"}

	accountCmd.AddCommand(&cobra.Command{
		Use:   "create",
		Short: "Create a new account",
		RunE: func(cmd *cobra.Command, args []string) error {
			c := client.NewClient(apiURL, "")
			resp, err := c.CreateAccount()
			if err != nil {
				return err
			}

			// Generate keypair
			privKey, pubKey, err := client.GenerateKeypair()
			if err != nil {
				return fmt.Errorf("generate keypair: %w", err)
			}

			cfg := &client.CLIConfig{
				APIURL:     apiURL,
				AccountID:  resp.AccountID,
				PrivateKey: privKey,
				PublicKey:  pubKey,
			}
			if err := client.SaveConfig(cfg); err != nil {
				return fmt.Errorf("save config: %w", err)
			}

			fmt.Printf("Account created: %s\n", resp.AccountID)
			fmt.Printf("Expires: %s\n", resp.ExpiresAt.Format("2006-01-02 15:04:05"))
			fmt.Printf("Config saved to: %s\n", client.ConfigPath())
			return nil
		},
	})

	accountCmd.AddCommand(&cobra.Command{
		Use:   "info",
		Short: "Show account info",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, c, err := loadClientFromConfig()
			if err != nil {
				return err
			}
			resp, err := c.GetAccount()
			if err != nil {
				return err
			}
			fmt.Printf("Account ID: %s\n", cfg.AccountID)
			fmt.Printf("Expires:    %s\n", resp.ExpiresAt.Format("2006-01-02 15:04:05"))
			fmt.Printf("Public Key: %s\n", cfg.PublicKey)
			return nil
		},
	})

	// ── Key commands ──
	keyCmd := &cobra.Command{Use: "key", Short: "Manage WireGuard keys"}

	keyCmd.AddCommand(&cobra.Command{
		Use:   "register",
		Short: "Register WireGuard public key with the API",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, c, err := loadClientFromConfig()
			if err != nil {
				return err
			}
			reg, err := c.RegisterKey(cfg.PublicKey)
			if err != nil {
				return err
			}
			fmt.Printf("Key registered!\n")
			fmt.Printf("  Server:     %s (%s)\n", reg.ServerID, reg.ServerIP)
			fmt.Printf("  Tunnel IP:  %s\n", reg.AllowedIP)
			fmt.Printf("  Server Key: %s\n", reg.ServerPubKey)
			return nil
		},
	})

	keyCmd.AddCommand(&cobra.Command{
		Use:   "list",
		Short: "List registered keys",
		RunE: func(cmd *cobra.Command, args []string) error {
			_, c, err := loadClientFromConfig()
			if err != nil {
				return err
			}
			keys, err := c.ListKeys()
			if err != nil {
				return err
			}
			if len(keys) == 0 {
				fmt.Println("No keys registered.")
				return nil
			}
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "ID\tPUBKEY\tSERVER\tTUNNEL IP")
			for _, k := range keys {
				short := k.PubKey[:20] + "..."
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", k.ID[:8], short, k.ServerID[:8], k.AllowedIP)
			}
			w.Flush()
			return nil
		},
	})

	keyCmd.AddCommand(&cobra.Command{
		Use:   "revoke [keyID]",
		Short: "Revoke a registered key",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			_, c, err := loadClientFromConfig()
			if err != nil {
				return err
			}
			if err := c.RevokeKey(args[0]); err != nil {
				return err
			}
			fmt.Println("Key revoked.")
			return nil
		},
	})

	// ── Server commands ──
	serverCmd := &cobra.Command{Use: "server", Short: "Browse VPN servers"}

	var regionFlag string
	listServersCmd := &cobra.Command{
		Use:   "list",
		Short: "List available servers",
		RunE: func(cmd *cobra.Command, args []string) error {
			_, c, err := loadClientFromConfig()
			if err != nil {
				return err
			}
			servers, err := c.ListServers(regionFlag)
			if err != nil {
				return err
			}
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "HOSTNAME\tREGION\tCOUNTRY\tSTATUS\tLOAD")
			for _, s := range servers {
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%.0f%%\n",
					s.Hostname, s.Region.City, s.Region.Country, s.Status, s.Load*100)
			}
			w.Flush()
			return nil
		},
	}
	listServersCmd.Flags().StringVar(&regionFlag, "region", "", "Filter by region code")
	serverCmd.AddCommand(listServersCmd)

	serverCmd.AddCommand(&cobra.Command{
		Use:   "ping",
		Short: "Test latency to all servers",
		RunE: func(cmd *cobra.Command, args []string) error {
			_, c, err := loadClientFromConfig()
			if err != nil {
				return err
			}
			servers, err := c.ListServers("")
			if err != nil {
				return err
			}
			fmt.Println("Pinging servers...")
			results := client.PingServers(servers)
			w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "HOSTNAME\tREGION\tLATENCY")
			for _, r := range results {
				if r.Error != nil {
					fmt.Fprintf(w, "%s\t%s\ttimeout\n", r.Server.Hostname, r.Server.Region.City)
				} else {
					fmt.Fprintf(w, "%s\t%s\t%s\n", r.Server.Hostname, r.Server.Region.City, r.Latency.Round(time.Millisecond))
				}
			}
			w.Flush()
			return nil
		},
	})

	// ── Connect / Disconnect ──
	var connectServer string
	connectCmd := &cobra.Command{
		Use:   "connect",
		Short: "Connect to VPN",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, c, err := loadClientFromConfig()
			if err != nil {
				return err
			}

			// Register key if not already
			reg, err := c.RegisterKey(cfg.PublicKey)
			if err != nil {
				if !strings.Contains(err.Error(), "duplicate") {
					return err
				}
				// Key already registered, get info
				keys, err := c.ListKeys()
				if err != nil {
					return err
				}
				if len(keys) == 0 {
					return fmt.Errorf("no keys registered")
				}
				// Use first key's server
				srv, err := c.ListServers("")
				if err != nil {
					return err
				}
				for _, s := range srv {
					if s.ID == keys[0].ServerID {
						reg = &client.KeyRegistration{
							ServerIP:     s.PublicIP,
							ServerPubKey: s.WGPubKey,
							ServerPort:   s.WGPort,
							AllowedIP:    keys[0].AllowedIP,
							ServerID:     keys[0].ServerID,
						}
						break
					}
				}
				if reg == nil {
					return fmt.Errorf("could not find server for registered key")
				}
			}

			_ = connectServer // TODO: server selection

			// Write WireGuard config
			wgDir := client.WGConfigDir()
			os.MkdirAll(wgDir, 0700)
			confPath := filepath.Join(wgDir, "wg-vpn.conf")

			dns := reg.ServerIP // Use server's Unbound
			conf := client.RenderWGConfig(cfg.PrivateKey, reg.ServerPubKey, reg.ServerIP, reg.ServerPort, reg.AllowedIP, dns)
			if err := os.WriteFile(confPath, []byte(conf), 0600); err != nil {
				return fmt.Errorf("write WireGuard config: %w", err)
			}

			fmt.Printf("Connecting to %s...\n", reg.ServerIP)
			if err := client.Connect("wg-vpn", confPath); err != nil {
				return err
			}

			cfg.Active = &client.ActiveConnection{
				ServerID:  reg.ServerID,
				Interface: "wg-vpn",
			}
			client.SaveConfig(cfg)

			fmt.Println("Connected!")
			return nil
		},
	}
	connectCmd.Flags().StringVar(&connectServer, "server", "", "Server hostname or ID")

	disconnectCmd := &cobra.Command{
		Use:   "disconnect",
		Short: "Disconnect from VPN",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, _, err := loadClientFromConfig()
			if err != nil {
				return err
			}
			iface := "wg-vpn"
			if cfg.Active != nil {
				iface = cfg.Active.Interface
			}
			if err := client.Disconnect(iface); err != nil {
				return err
			}
			cfg.Active = nil
			client.SaveConfig(cfg)
			fmt.Println("Disconnected.")
			return nil
		},
	}

	statusCmd := &cobra.Command{
		Use:   "status",
		Short: "Show connection status",
		RunE: func(cmd *cobra.Command, args []string) error {
			s, err := client.Status("wg-vpn")
			if err != nil {
				return err
			}
			if !s.Connected {
				fmt.Println("Not connected.")
				return nil
			}
			fmt.Printf("Interface: %s\n", s.Interface)
			fmt.Printf("Endpoint:  %s\n", s.Endpoint)
			fmt.Printf("Transfer:  %s\n", s.Transfer)
			return nil
		},
	}

	// ── Multihop ──
	var entryFlag, exitFlag string
	multihopCmd := &cobra.Command{Use: "multihop", Short: "Multi-hop VPN"}
	mhConnectCmd := &cobra.Command{
		Use:   "connect",
		Short: "Connect via two-hop VPN",
		RunE: func(cmd *cobra.Command, args []string) error {
			if entryFlag == "" || exitFlag == "" {
				return fmt.Errorf("--entry and --exit are required")
			}
			cfg, c, err := loadClientFromConfig()
			if err != nil {
				return err
			}

			servers, err := c.ListServers("")
			if err != nil {
				return err
			}

			var entry, exit *client.ServerInfo
			for i := range servers {
				if servers[i].Hostname == entryFlag || servers[i].ID == entryFlag {
					entry = &servers[i]
				}
				if servers[i].Hostname == exitFlag || servers[i].ID == exitFlag {
					exit = &servers[i]
				}
			}
			if entry == nil {
				return fmt.Errorf("entry server %q not found", entryFlag)
			}
			if exit == nil {
				return fmt.Errorf("exit server %q not found", exitFlag)
			}

			// Register key on both servers (for simplicity, use same key)
			entryReg, err := c.RegisterKey(cfg.PublicKey)
			if err != nil {
				return fmt.Errorf("register entry key: %w", err)
			}

			// Generate second keypair for exit
			privKey2, pubKey2, err := client.GenerateKeypair()
			if err != nil {
				return fmt.Errorf("generate exit keypair: %w", err)
			}
			exitReg, err := c.RegisterKey(pubKey2)
			if err != nil {
				return fmt.Errorf("register exit key: %w", err)
			}

			wgDir := client.WGConfigDir()
			os.MkdirAll(wgDir, 0700)

			entryConf, exitConf := client.RenderMultihopConfigs(
				cfg.PrivateKey, *entry, *exit, entryReg.AllowedIP, exitReg.AllowedIP,
			)

			// Write entry config using main private key
			entryPath := filepath.Join(wgDir, "wg-vpn0.conf")
			os.WriteFile(entryPath, []byte(entryConf), 0600)

			// Write exit config using second private key
			exitConf = strings.Replace(exitConf, cfg.PrivateKey, privKey2, 1)
			exitPath := filepath.Join(wgDir, "wg-vpn1.conf")
			os.WriteFile(exitPath, []byte(exitConf), 0600)

			fmt.Printf("Connecting entry hop: %s (%s)...\n", entry.Hostname, entry.PublicIP)
			if err := client.Connect("wg-vpn0", entryPath); err != nil {
				return fmt.Errorf("entry connect: %w", err)
			}

			fmt.Printf("Connecting exit hop: %s (%s)...\n", exit.Hostname, exit.PublicIP)
			if err := client.Connect("wg-vpn1", exitPath); err != nil {
				client.Disconnect("wg-vpn0")
				return fmt.Errorf("exit connect: %w", err)
			}

			cfg.Active = &client.ActiveConnection{
				ServerID:  exit.ID,
				Interface: "wg-vpn1",
			}
			client.SaveConfig(cfg)

			fmt.Println("Multi-hop connected!")
			return nil
		},
	}
	mhConnectCmd.Flags().StringVar(&entryFlag, "entry", "", "Entry server hostname/ID")
	mhConnectCmd.Flags().StringVar(&exitFlag, "exit", "", "Exit server hostname/ID")
	multihopCmd.AddCommand(mhConnectCmd)

	root.AddCommand(accountCmd, keyCmd, serverCmd, connectCmd, disconnectCmd, statusCmd, multihopCmd)

	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}

func loadClientFromConfig() (*client.CLIConfig, *client.Client, error) {
	cfg, err := client.LoadConfig()
	if err != nil {
		return nil, nil, err
	}
	if cfg.AccountID == "" {
		return nil, nil, fmt.Errorf("no account configured — run 'vpncli account create' first")
	}
	url := apiURL
	if cfg.APIURL != "" {
		url = cfg.APIURL
	}
	return cfg, client.NewClient(url, cfg.AccountID), nil
}
