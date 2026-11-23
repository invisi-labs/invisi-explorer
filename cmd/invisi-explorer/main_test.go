package main

import (
	"bytes"
	"context"
	"path/filepath"
	"strings"
	"testing"

	"github.com/invisi-labs/invisi-explorer/internal/config"
)

func invoke(t *testing.T, dir string, args ...string) (string, error) {
	t.Helper()
	var out bytes.Buffer
	err := newCommand(&out, &out).Run(context.Background(), append([]string{"invisi-explorer", "--dir", dir}, args...))
	return out.String(), err
}
func TestCLIManagesExplicitPublicConfiguration(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "explorer")
	if _, err := invoke(t, dir, "init"); err != nil {
		t.Fatal(err)
	}
	addr := "0x1111111111111111111111111111111111111111"
	if _, err := invoke(t, dir, "chain", "add", "--chain-id", "1", "--protocol-address", addr, "--rpc-url", "http://127.0.0.1:18545", "--start-block", "42", "--name", "Ethereum", "--environment", "mainnet", "--explorer-url", "https://etherscan.io"); err != nil {
		t.Fatal(err)
	}
	if _, err := invoke(t, dir, "token", "add", "--chain-id", "1", "--address", "0x2222222222222222222222222222222222222222", "--symbol", "USDC", "--decimals", "6"); err != nil {
		t.Fatal(err)
	}
	if _, err := invoke(t, dir, "settings", "set", "--poll-interval", "8s"); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Chains) != 1 || cfg.Chains[0].StartBlock != 42 || len(cfg.Tokens) != 1 || cfg.Tokens[0].Decimals != 6 || cfg.PollInterval != "8s" || cfg.Chains[0].Name != "Ethereum" || cfg.Chains[0].ExplorerURL != "https://etherscan.io" {
		t.Fatalf("wrong persisted config: %+v", cfg)
	}
	out, err := invoke(t, dir, "chain", "list")
	if err != nil || !strings.Contains(out, addr) {
		t.Fatalf("list: %s %v", out, err)
	}
	if _, err := invoke(t, dir, "chain", "remove", "--chain-id", "1", "--protocol-address", addr); err == nil {
		t.Fatal("removed chain referenced by token")
	}
	if _, err := invoke(t, dir, "token", "remove", "--chain-id", "1", "--address", "0x2222222222222222222222222222222222222222"); err != nil {
		t.Fatal(err)
	}
	if _, err := invoke(t, dir, "chain", "remove", "--chain-id", "1", "--protocol-address", addr); err != nil {
		t.Fatal(err)
	}
}
func TestCLIRejectsUnknownCommandsAndInvalidMutation(t *testing.T) {
	dir := t.TempDir()
	if _, err := invoke(t, dir, "init"); err != nil {
		t.Fatal(err)
	}
	for _, args := range [][]string{{"chain", "discover"}, {"settings", "set", "--poll-interval", "-1s"}, {"chain", "add", "--chain-id", "1", "--protocol-address", "0x1111111111111111111111111111111111111111", "--rpc-url", "file:///tmp/socket"}, {"token", "add", "--chain-id", "1", "--address", "0x2222222222222222222222222222222222222222", "--symbol", "T", "--decimals", "256"}, {"run", "extra"}} {
		if _, err := invoke(t, dir, args...); err == nil {
			t.Fatalf("accepted %v", args)
		}
	}
}

func TestCLIUpdatesScopeEndpointsWithoutRemovingTokensOrPins(t *testing.T) {
	dir := t.TempDir()
	if _, err := invoke(t, dir, "init"); err != nil {
		t.Fatal(err)
	}
	addr := "0x1111111111111111111111111111111111111111"
	if _, err := invoke(t, dir, "chain", "add", "--chain-id", "97", "--protocol-address", addr, "--rpc-url", "https://rpc.example/v1/chains/97", "--start-block", "42"); err != nil {
		t.Fatal(err)
	}
	if _, err := invoke(t, dir, "token", "add", "--chain-id", "97", "--address", addr, "--symbol", "T", "--decimals", "18"); err != nil {
		t.Fatal(err)
	}
	selectScope := []string{"--chain-id", "97", "--protocol-address", addr}
	args := append([]string{"chain", "set"}, selectScope...)
	args = append(args, "--broadcaster-url", "https://broadcaster.example/v1/chains/97", "--name", "BNB Testnet")
	if _, err := invoke(t, dir, args...); err != nil {
		t.Fatal(err)
	}
	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(cfg.Tokens) != 1 || cfg.Chains[0].StartBlock != 42 || cfg.Chains[0].BroadcasterURL != "https://broadcaster.example/v1/chains/97" || cfg.Chains[0].Name != "BNB Testnet" {
		t.Fatalf("scope changed incorrectly: %+v", cfg)
	}
	args = append([]string{"chain", "show"}, selectScope...)
	if output, err := invoke(t, dir, args...); err != nil || !strings.Contains(output, "BNB Testnet") {
		t.Fatalf("scope show: %s %v", output, err)
	}
	args = append([]string{"chain", "set"}, selectScope...)
	args = append(args, "--rpc-url", "file:///tmp/private")
	if _, err := invoke(t, dir, args...); err == nil {
		t.Fatal("accepted invalid replacement")
	}
	cfg, _ = config.Load(dir)
	if cfg.Chains[0].RPCURL != "https://rpc.example/v1/chains/97" {
		t.Fatal("failed edit corrupted scope")
	}
	args = append([]string{"chain", "set"}, selectScope...)
	if _, err := invoke(t, dir, args...); err == nil {
		t.Fatal("accepted empty edit")
	}
}
