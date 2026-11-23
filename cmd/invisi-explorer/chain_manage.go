package main

import (
	"context"
	"errors"
	"strings"

	"github.com/invisi-labs/invisi-explorer/internal/config"
	"github.com/urfave/cli/v3"
)

func selectedChain(cfg *config.Config, cmd *cli.Command) (*config.Chain, error) {
	for i := range cfg.Chains {
		chain := &cfg.Chains[i]
		if chain.ChainID == cmd.String("chain-id") && strings.EqualFold(chain.ProtocolAddress, cmd.String("protocol-address")) {
			return chain, nil
		}
	}
	return nil, errors.New("chain/protocol scope not configured")
}

func chainShowCommand() *cli.Command {
	return &cli.Command{Name: "show", Usage: "Show a configured chain/protocol scope", Flags: scopeFlags(), Action: func(_ context.Context, cmd *cli.Command) error {
		if err := noArgs(cmd); err != nil {
			return err
		}
		cfg, err := config.Load(dataDir(cmd))
		if err != nil {
			return err
		}
		chain, err := selectedChain(&cfg, cmd)
		if err != nil {
			return err
		}
		return printJSON(cmd, chain)
	}}
}

func chainSetCommand() *cli.Command {
	flags := append(scopeFlags(), &cli.StringFlag{Name: "rpc-url"}, &cli.StringFlag{Name: "provenance-url"}, &cli.StringFlag{Name: "broadcaster-url"}, &cli.StringFlag{Name: "name"}, &cli.StringFlag{Name: "environment"}, &cli.StringFlag{Name: "explorer-url"})
	return &cli.Command{Name: "set", Usage: "Update endpoints/display metadata while preserving deployment pins (restart required)", Flags: flags, Action: func(_ context.Context, cmd *cli.Command) error {
		return mutate(cmd, func(cfg *config.Config) error {
			chain, err := selectedChain(cfg, cmd)
			if err != nil {
				return err
			}
			changed := false
			for _, field := range []struct {
				name  string
				value *string
			}{
				{"rpc-url", &chain.RPCURL}, {"provenance-url", &chain.ProvenanceURL}, {"broadcaster-url", &chain.BroadcasterURL},
				{"name", &chain.Name}, {"environment", &chain.Environment}, {"explorer-url", &chain.ExplorerURL},
			} {
				if cmd.IsSet(field.name) {
					*field.value = cmd.String(field.name)
					changed = true
				}
			}
			if !changed {
				return errors.New("specify at least one endpoint or display setting")
			}
			return nil
		})
	}}
}
