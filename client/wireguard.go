package client

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"strings"

	"golang.org/x/crypto/curve25519"
)

func GenerateKeypair() (privateKey, publicKey string, err error) {
	var privKey [32]byte
	if _, err := rand.Read(privKey[:]); err != nil {
		return "", "", fmt.Errorf("generate private key: %w", err)
	}

	// Clamp the private key per WireGuard spec
	privKey[0] &= 248
	privKey[31] &= 127
	privKey[31] |= 64

	pubKey, err := curve25519.X25519(privKey[:], curve25519.Basepoint)
	if err != nil {
		return "", "", fmt.Errorf("derive public key: %w", err)
	}

	return base64.StdEncoding.EncodeToString(privKey[:]),
		base64.StdEncoding.EncodeToString(pubKey), nil
}

func RenderWGConfig(privateKey, serverPubkey, serverEndpoint string, serverPort int, allowedIP, dns string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "[Interface]\n")
	fmt.Fprintf(&b, "PrivateKey = %s\n", privateKey)
	// Strip the /32 for the Address field
	addr := strings.TrimSuffix(allowedIP, "/32")
	fmt.Fprintf(&b, "Address = %s/32\n", addr)
	fmt.Fprintf(&b, "DNS = %s\n", dns)
	fmt.Fprintf(&b, "\n")
	fmt.Fprintf(&b, "[Peer]\n")
	fmt.Fprintf(&b, "PublicKey = %s\n", serverPubkey)
	fmt.Fprintf(&b, "AllowedIPs = 0.0.0.0/0, ::/0\n")
	fmt.Fprintf(&b, "Endpoint = %s:%d\n", serverEndpoint, serverPort)
	fmt.Fprintf(&b, "PersistentKeepalive = 25\n")
	return b.String()
}

func RenderMultihopConfigs(privateKey string, entry, exit ServerInfo, entryIP, exitIP string) (entryConf, exitConf string) {
	var eb strings.Builder
	fmt.Fprintf(&eb, "[Interface]\n")
	fmt.Fprintf(&eb, "PrivateKey = %s\n", privateKey)
	entryAddr := strings.TrimSuffix(entryIP, "/32")
	fmt.Fprintf(&eb, "Address = %s/32\n", entryAddr)
	fmt.Fprintf(&eb, "\n")
	fmt.Fprintf(&eb, "[Peer]\n")
	fmt.Fprintf(&eb, "PublicKey = %s\n", entry.WGPubKey)
	// Route only the exit server's IP through the entry hop
	fmt.Fprintf(&eb, "AllowedIPs = %s/32\n", exit.PublicIP)
	fmt.Fprintf(&eb, "Endpoint = %s:%d\n", entry.PublicIP, entry.WGPort)
	fmt.Fprintf(&eb, "PersistentKeepalive = 25\n")
	entryConf = eb.String()

	var xb strings.Builder
	fmt.Fprintf(&xb, "[Interface]\n")
	fmt.Fprintf(&xb, "PrivateKey = %s\n", privateKey)
	exitAddr := strings.TrimSuffix(exitIP, "/32")
	fmt.Fprintf(&xb, "Address = %s/32\n", exitAddr)
	fmt.Fprintf(&xb, "DNS = %s\n", exit.PublicIP)
	fmt.Fprintf(&xb, "\n")
	fmt.Fprintf(&xb, "[Peer]\n")
	fmt.Fprintf(&xb, "PublicKey = %s\n", exit.WGPubKey)
	fmt.Fprintf(&xb, "AllowedIPs = 0.0.0.0/0, ::/0\n")
	fmt.Fprintf(&xb, "Endpoint = %s:%d\n", exit.PublicIP, exit.WGPort)
	fmt.Fprintf(&xb, "PersistentKeepalive = 25\n")
	exitConf = xb.String()

	return entryConf, exitConf
}
