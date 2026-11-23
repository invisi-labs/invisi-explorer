// Package config manages explicit public explorer configuration. It stores no signing material.
package config

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const DefaultListen = "127.0.0.1:18670"
const Filename = "config.json"

type Chain struct {
	Name            string `json:"name,omitempty"`
	Environment     string `json:"environment,omitempty"`
	ExplorerURL     string `json:"explorerUrl,omitempty"`
	ChainID         string `json:"chainId"`
	ProtocolAddress string `json:"protocolAddress"`
	RPCURL          string `json:"rpcUrl"`
	ProvenanceURL   string `json:"provenanceUrl,omitempty"`
	CodeHash        string `json:"codeHash,omitempty"`
	StartBlock      uint64 `json:"startBlock,omitempty"`
	BroadcasterURL  string `json:"broadcasterUrl,omitempty"`
	IssuerID        string `json:"issuerId,omitempty"`
}
type Token struct {
	ChainID  string `json:"chainId"`
	Address  string `json:"address"`
	Symbol   string `json:"symbol"`
	Name     string `json:"name,omitempty"`
	Decimals uint8  `json:"decimals"`
}
type Config struct {
	Listen       string  `json:"listen"`
	Database     string  `json:"database"`
	PollInterval string  `json:"pollInterval"`
	WebDir       string  `json:"webDir"`
	Chains       []Chain `json:"chains"`
	Tokens       []Token `json:"tokens"`
}

func Default() Config {
	return Config{Listen: DefaultListen, Database: "explorer.sqlite", PollInterval: "5s", WebDir: "web/dist", Chains: []Chain{}, Tokens: []Token{}}
}
func (c Config) PollDuration() time.Duration { d, _ := time.ParseDuration(c.PollInterval); return d }
func ValidateListen(s string) error {
	host, port, err := net.SplitHostPort(s)
	if err != nil || host == "" {
		return errors.New("listen must be a host:port address")
	}
	p, err := strconv.Atoi(port)
	if err != nil || p < 1 || p > 65535 {
		return errors.New("listen port must be between 1 and 65535")
	}
	return nil
}
func ValidateEndpoint(s string) error {
	u, err := url.Parse(s)
	if err != nil || u.Hostname() == "" || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil || u.Fragment != "" {
		return errors.New("endpoint must be an explicit HTTP/S URL without user credentials or fragment")
	}
	return nil
}
func ValidateChainID(s string) error {
	n, ok := new(big.Int).SetString(s, 10)
	if !ok || n.Sign() <= 0 || n.BitLen() > 64 || n.String() != s {
		return errors.New("chainId must be a positive canonical decimal uint64")
	}
	return nil
}
func ValidateAddress(s string) error { return validateHex(s, 20, "address") }
func validateHex(s string, n int, label string) error {
	if !strings.HasPrefix(s, "0x") || len(s) != 2+n*2 {
		return fmt.Errorf("%s must be 0x-prefixed %d-byte hex", label, n)
	}
	_, err := hex.DecodeString(s[2:])
	if err != nil {
		return fmt.Errorf("invalid %s: %w", label, err)
	}
	return nil
}
func (c Config) Validate() error {
	if err := ValidateListen(c.Listen); err != nil {
		return err
	}
	if strings.TrimSpace(c.Database) == "" {
		return errors.New("database must not be empty")
	}
	d, err := time.ParseDuration(c.PollInterval)
	if err != nil || d <= 0 {
		return errors.New("pollInterval must be a positive duration")
	}
	scopes := map[string]bool{}
	chains := map[string]bool{}
	for _, ch := range c.Chains {
		if err := ValidateChainID(ch.ChainID); err != nil {
			return err
		}
		if err := ValidateAddress(ch.ProtocolAddress); err != nil {
			return fmt.Errorf("protocolAddress: %w", err)
		}
		if ch.ProtocolAddress == "0x0000000000000000000000000000000000000000" {
			return errors.New("protocolAddress must not be zero")
		}
		if err := ValidateEndpoint(ch.RPCURL); err != nil {
			return fmt.Errorf("rpcUrl: %w", err)
		}
		for _, endpoint := range []string{ch.ProvenanceURL, ch.BroadcasterURL, ch.ExplorerURL} {
			if endpoint != "" {
				if err := ValidateEndpoint(endpoint); err != nil {
					return err
				}
			}
		}
		if ch.CodeHash != "" {
			if err := validateHex(ch.CodeHash, 32, "codeHash"); err != nil {
				return err
			}
		}
		key := ch.ChainID + ":" + strings.ToLower(ch.ProtocolAddress)
		if scopes[key] {
			return errors.New("duplicate chain/protocol scope")
		}
		scopes[key] = true
		chains[ch.ChainID] = true
	}
	tokens := map[string]bool{}
	for _, t := range c.Tokens {
		if !chains[t.ChainID] {
			return fmt.Errorf("token references unconfigured chain %q", t.ChainID)
		}
		if err := ValidateAddress(t.Address); err != nil {
			return err
		}
		if strings.TrimSpace(t.Symbol) == "" {
			return errors.New("token symbol must not be empty")
		}
		key := t.ChainID + ":" + strings.ToLower(t.Address)
		if tokens[key] {
			return errors.New("duplicate token")
		}
		tokens[key] = true
	}
	return nil
}
func Load(dir string) (Config, error) {
	f, err := os.Open(filepath.Join(dir, Filename))
	if err != nil {
		return Config{}, err
	}
	defer f.Close()
	c := Default()
	body, err := io.ReadAll(io.LimitReader(f, (1<<20)+1))
	if err != nil {
		return Config{}, err
	}
	if len(body) > 1<<20 {
		return Config{}, errors.New("configuration exceeds 1 MiB")
	}
	dec := json.NewDecoder(bytes.NewReader(body))
	dec.DisallowUnknownFields()
	if err = dec.Decode(&c); err != nil {
		return Config{}, err
	}
	if err = dec.Decode(new(any)); err != io.EOF {
		return Config{}, errors.New("configuration contains trailing JSON or exceeds 1 MiB")
	}
	return c, c.Validate()
}
func Init(dir string) error {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	// Publish a complete file atomically, with link's exclusive destination semantics
	// preserving an existing operator configuration even when init races another process.
	f, err := os.CreateTemp(dir, ".config-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	defer f.Close()
	if err = json.NewEncoder(f).Encode(Default()); err != nil {
		return err
	}
	if err = f.Sync(); err != nil {
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return os.Link(f.Name(), filepath.Join(dir, Filename))
}
func Save(dir string, c Config) error {
	if err := c.Validate(); err != nil {
		return err
	}
	if err := os.MkdirAll(dir, 0700); err != nil {
		return err
	}
	f, err := os.CreateTemp(dir, ".config-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	defer f.Close()
	enc := json.NewEncoder(f)
	enc.SetIndent("", "  ")
	if err = enc.Encode(c); err != nil {
		return err
	}
	if err = f.Sync(); err != nil {
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return os.Rename(f.Name(), filepath.Join(dir, Filename))
}
