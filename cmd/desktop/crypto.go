package main

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"os/exec"
	"strings"

	"golang.org/x/crypto/curve25519"
)

func generateKeypair() (privateKey, publicKey string, err error) {
	// Try native wg command first
	if priv, pub, err := generateKeypairWG(); err == nil {
		return priv, pub, nil
	}

	// Pure Go fallback
	return generateKeypairGo()
}

func generateKeypairWG() (string, string, error) {
	genOut, err := exec.Command("wg", "genkey").Output()
	if err != nil {
		return "", "", err
	}
	priv := strings.TrimSpace(string(genOut))

	cmd := exec.Command("wg", "pubkey")
	cmd.Stdin = strings.NewReader(priv)
	pubOut, err := cmd.Output()
	if err != nil {
		return "", "", err
	}

	return priv, strings.TrimSpace(string(pubOut)), nil
}

func generateKeypairGo() (string, string, error) {
	var privBytes [32]byte
	if _, err := rand.Read(privBytes[:]); err != nil {
		return "", "", fmt.Errorf("random: %w", err)
	}

	privBytes[0] &= 248
	privBytes[31] &= 127
	privBytes[31] |= 64

	pubBytes, err := curve25519.X25519(privBytes[:], curve25519.Basepoint)
	if err != nil {
		return "", "", fmt.Errorf("x25519: %w", err)
	}

	return base64.StdEncoding.EncodeToString(privBytes[:]),
		base64.StdEncoding.EncodeToString(pubBytes), nil
}
