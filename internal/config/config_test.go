package config

import (
	"os"
	"path/filepath"
	"testing"
)

func validChain() Chain {
	return Chain{ChainID: "1", ProtocolAddress: "0x1111111111111111111111111111111111111111", RPCURL: "http://127.0.0.1:18545"}
}
func TestInitPersistsPrivateConfigWithoutOverwriting(t *testing.T) {
	dir := t.TempDir()
	if err := Init(dir); err != nil {
		t.Fatal(err)
	}
	cfg, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	cfg.Listen = "127.0.0.1:19000"
	if err = Save(dir, cfg); err != nil {
		t.Fatal(err)
	}
	if err = Init(dir); err == nil {
		t.Fatal("init overwrote existing configuration")
	}
	cfg, err = Load(dir)
	if err != nil || cfg.Listen != "127.0.0.1:19000" {
		t.Fatalf("config lost: %+v %v", cfg, err)
	}
	st, err := os.Stat(filepath.Join(dir, "config.json"))
	if err != nil {
		t.Fatal(err)
	}
	if st.Mode().Perm() != 0600 {
		t.Fatalf("permissions: %o", st.Mode().Perm())
	}
}
func TestSaveRejectsInvalidConfigWithoutReplacingPriorFile(t *testing.T) {
	dir := t.TempDir()
	if err := Init(dir); err != nil {
		t.Fatal(err)
	}
	cfg, _ := Load(dir)
	cfg.Chains = []Chain{validChain()}
	if err := Save(dir, cfg); err != nil {
		t.Fatal(err)
	}
	cfg.Chains[0].RPCURL = "file:///etc/passwd"
	if err := Save(dir, cfg); err == nil {
		t.Fatal("accepted non HTTP endpoint")
	}
	got, err := Load(dir)
	if err != nil || got.Chains[0].RPCURL != "http://127.0.0.1:18545" {
		t.Fatalf("prior configuration corrupted: %+v %v", got, err)
	}
}
func TestValidateScopeAndEndpoints(t *testing.T) {
	for _, tc := range []struct {
		name string
		edit func(*Config)
	}{
		{"overflowing chain", func(c *Config) { c.Chains[0].ChainID = "18446744073709551616" }},
		{"noncanonical chain", func(c *Config) { c.Chains[0].ChainID = "01" }},
		{"zero protocol", func(c *Config) { c.Chains[0].ProtocolAddress = "0x0000000000000000000000000000000000000000" }},
		{"bad protocol", func(c *Config) { c.Chains[0].ProtocolAddress = "0x123" }},
		{"credential endpoint", func(c *Config) { c.Chains[0].RPCURL = "https://user:password@example.com" }},
		{"bad explorer", func(c *Config) { c.Chains[0].ExplorerURL = "javascript:alert(1)" }},
		{"bad provenance", func(c *Config) { c.Chains[0].ProvenanceURL = "wss://example.com" }},
		{"duplicate scope", func(c *Config) { c.Chains = append(c.Chains, c.Chains[0]) }},
		{"poll interval", func(c *Config) { c.PollInterval = "0s" }},
		{"token unknown chain", func(c *Config) {
			c.Tokens = []Token{{ChainID: "2", Address: "0x2222222222222222222222222222222222222222", Symbol: "T", Decimals: 18}}
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cfg := Default()
			cfg.Chains = []Chain{validChain()}
			tc.edit(&cfg)
			if err := cfg.Validate(); err == nil {
				t.Fatal("accepted invalid config")
			}
		})
	}
}
func TestLoadRejectsUnknownFieldsAndTrailingJSON(t *testing.T) {
	for _, body := range []string{`{"privateKey":"secret"}`, `{} {}`} {
		dir := t.TempDir()
		if err := os.WriteFile(filepath.Join(dir, "config.json"), []byte(body), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := Load(dir); err == nil {
			t.Fatalf("accepted %s", body)
		}
	}
}

func TestLoadRejectsOversizedConfigurationEvenWhenPrefixIsValid(t *testing.T) {
	dir := t.TempDir()
	body := append([]byte("{}"), make([]byte, 1<<20)...)
	for i := 2; i < len(body); i++ {
		body[i] = ' '
	}
	if err := os.WriteFile(filepath.Join(dir, Filename), body, 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(dir); err == nil {
		t.Fatal("accepted oversized configuration after valid JSON prefix")
	}
}
