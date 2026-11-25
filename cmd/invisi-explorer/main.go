package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/invisi-labs/invisi-explorer/internal/config"
	"github.com/invisi-labs/invisi-explorer/internal/service"
	"github.com/urfave/cli/v3"
)

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := newCommand(os.Stdout, os.Stderr).Run(ctx, os.Args); err != nil {
		fmt.Fprintln(os.Stderr, "invisi-explorer:", err)
		os.Exit(1)
	}
}
func dataDir(cmd *cli.Command) string { return cmd.String("dir") }
func noArgs(cmd *cli.Command) error {
	if cmd.Args().Present() {
		return fmt.Errorf("%s accepts no positional arguments", cmd.Name)
	}
	return nil
}
func printJSON(cmd *cli.Command, v any) error {
	enc := json.NewEncoder(cmd.Writer)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}
func newCommand(out, errOut io.Writer) *cli.Command {
	return &cli.Command{Name: "invisi-explorer", Version: "1.3.8", Usage: "Run and configure an explorer for explicit public Invisi scopes", Writer: out, ErrWriter: errOut,
		Flags: []cli.Flag{&cli.StringFlag{Name: "dir", Usage: "Configuration and database directory", Value: "./invisi-explorer", Sources: cli.EnvVars("INVISI_EXPLORER_DATA_DIR")}},
		Action: func(_ context.Context, cmd *cli.Command) error {
			if cmd.Args().Present() {
				return fmt.Errorf("unknown command %q", cmd.Args().First())
			}
			return cli.ShowRootCommandHelp(cmd)
		},
		Commands: []*cli.Command{
			{Name: "init", Usage: "Initialize a public configuration file (no keys)", Flags: []cli.Flag{&cli.StringFlag{Name: "listen", Value: config.DefaultListen}}, Action: func(_ context.Context, cmd *cli.Command) error {
				if err := noArgs(cmd); err != nil {
					return err
				}
				if err := config.ValidateListen(cmd.String("listen")); err != nil {
					return err
				}
				if err := config.Init(dataDir(cmd)); err != nil {
					return err
				}
				if cmd.IsSet("listen") {
					cfg, err := config.Load(dataDir(cmd))
					if err != nil {
						return err
					}
					cfg.Listen = cmd.String("listen")
					return config.Save(dataDir(cmd), cfg)
				}
				return nil
			}},
			{Name: "run", Usage: "Serve the explorer API, indexer, and built frontend", Flags: []cli.Flag{&cli.StringFlag{Name: "listen", Usage: "Listen address override (not saved)"}}, Action: func(ctx context.Context, cmd *cli.Command) error {
				if err := noArgs(cmd); err != nil {
					return err
				}
				cfg, err := config.Load(dataDir(cmd))
				if err != nil {
					return err
				}
				if cmd.IsSet("listen") {
					cfg.Listen = cmd.String("listen")
				}
				if err = cfg.Validate(); err != nil {
					return err
				}
				if !filepath.IsAbs(cfg.Database) {
					cfg.Database = filepath.Join(dataDir(cmd), cfg.Database)
				}
				return service.Run(ctx, cfg)
			}},
			settingsCommand(), chainCommand(), tokenCommand(),
		}}
}
func settingsCommand() *cli.Command {
	return &cli.Command{Name: "settings", Usage: "Inspect or update local settings", Action: groupHelp, Commands: []*cli.Command{
		{Name: "show", Action: func(_ context.Context, cmd *cli.Command) error {
			if err := noArgs(cmd); err != nil {
				return err
			}
			cfg, err := config.Load(dataDir(cmd))
			if err != nil {
				return err
			}
			return printJSON(cmd, cfg)
		}},
		{Name: "set", Usage: "Persist selected settings", Flags: []cli.Flag{&cli.StringFlag{Name: "listen"}, &cli.StringFlag{Name: "database"}, &cli.StringFlag{Name: "poll-interval"}, &cli.StringFlag{Name: "web-dir"}}, Action: func(_ context.Context, cmd *cli.Command) error {
			return mutate(cmd, func(cfg *config.Config) error {
				changed := false
				for _, field := range []struct {
					name   string
					target *string
				}{{"listen", &cfg.Listen}, {"database", &cfg.Database}, {"poll-interval", &cfg.PollInterval}, {"web-dir", &cfg.WebDir}} {
					if cmd.IsSet(field.name) {
						*field.target = cmd.String(field.name)
						changed = true
					}
				}
				if !changed {
					return errors.New("specify at least one setting")
				}
				return nil
			})
		}},
	}}
}
func groupHelp(_ context.Context, cmd *cli.Command) error {
	if cmd.Args().Present() {
		return fmt.Errorf("unknown %s command %q", cmd.Name, cmd.Args().First())
	}
	return cli.ShowSubcommandHelp(cmd)
}
func mutate(cmd *cli.Command, edit func(*config.Config) error) error {
	if err := noArgs(cmd); err != nil {
		return err
	}
	cfg, err := config.Load(dataDir(cmd))
	if err != nil {
		return err
	}
	if err = edit(&cfg); err != nil {
		return err
	}
	return config.Save(dataDir(cmd), cfg)
}
func scopeFlags() []cli.Flag {
	return []cli.Flag{&cli.StringFlag{Name: "chain-id", Required: true}, &cli.StringFlag{Name: "protocol-address", Required: true}}
}
func chainCommand() *cli.Command {
	flags := append(scopeFlags(), &cli.StringFlag{Name: "rpc-url", Required: true}, &cli.StringFlag{Name: "provenance-url"}, &cli.StringFlag{Name: "broadcaster-url"}, &cli.StringFlag{Name: "code-hash"}, &cli.Uint64Flag{Name: "start-block"}, &cli.StringFlag{Name: "issuer-id"}, &cli.StringFlag{Name: "name"}, &cli.StringFlag{Name: "environment"}, &cli.StringFlag{Name: "explorer-url"})
	return &cli.Command{Name: "chain", Usage: "Manage explicitly configured chain/protocol scopes", Action: groupHelp, Commands: []*cli.Command{
		chainShowCommand(), chainSetCommand(),
		{Name: "list", Action: func(_ context.Context, cmd *cli.Command) error {
			if err := noArgs(cmd); err != nil {
				return err
			}
			cfg, err := config.Load(dataDir(cmd))
			if err != nil {
				return err
			}
			return printJSON(cmd, cfg.Chains)
		}},
		{Name: "add", Flags: flags, Action: func(_ context.Context, cmd *cli.Command) error {
			return mutate(cmd, func(cfg *config.Config) error {
				cfg.Chains = append(cfg.Chains, config.Chain{Name: cmd.String("name"), Environment: cmd.String("environment"), ExplorerURL: cmd.String("explorer-url"), ChainID: cmd.String("chain-id"), ProtocolAddress: cmd.String("protocol-address"), RPCURL: cmd.String("rpc-url"), ProvenanceURL: cmd.String("provenance-url"), BroadcasterURL: cmd.String("broadcaster-url"), CodeHash: cmd.String("code-hash"), StartBlock: cmd.Uint64("start-block"), IssuerID: cmd.String("issuer-id")})
				return nil
			})
		}},
		{Name: "remove", Flags: scopeFlags(), Action: func(_ context.Context, cmd *cli.Command) error {
			return mutate(cmd, func(cfg *config.Config) error {
				for i, ch := range cfg.Chains {
					if ch.ChainID == cmd.String("chain-id") && strings.EqualFold(ch.ProtocolAddress, cmd.String("protocol-address")) {
						cfg.Chains = append(cfg.Chains[:i], cfg.Chains[i+1:]...)
						return nil
					}
				}
				return errors.New("chain/protocol scope not configured")
			})
		}},
	}}
}
func tokenFlags() []cli.Flag {
	return []cli.Flag{&cli.StringFlag{Name: "chain-id", Required: true}, &cli.StringFlag{Name: "address", Required: true}}
}
func tokenCommand() *cli.Command {
	flags := append(tokenFlags(), &cli.StringFlag{Name: "symbol", Required: true}, &cli.StringFlag{Name: "name"}, &cli.UintFlag{Name: "decimals", Required: true})
	return &cli.Command{Name: "token", Usage: "Manage explicit token metadata; no token discovery", Action: groupHelp, Commands: []*cli.Command{
		{Name: "list", Action: func(_ context.Context, cmd *cli.Command) error {
			if err := noArgs(cmd); err != nil {
				return err
			}
			cfg, err := config.Load(dataDir(cmd))
			if err != nil {
				return err
			}
			return printJSON(cmd, cfg.Tokens)
		}},
		{Name: "add", Flags: flags, Action: func(_ context.Context, cmd *cli.Command) error {
			if cmd.Uint("decimals") > 255 {
				return errors.New("decimals must be between 0 and 255")
			}
			return mutate(cmd, func(cfg *config.Config) error {
				cfg.Tokens = append(cfg.Tokens, config.Token{ChainID: cmd.String("chain-id"), Address: cmd.String("address"), Symbol: cmd.String("symbol"), Name: cmd.String("name"), Decimals: uint8(cmd.Uint("decimals"))})
				return nil
			})
		}},
		{Name: "remove", Flags: tokenFlags(), Action: func(_ context.Context, cmd *cli.Command) error {
			return mutate(cmd, func(cfg *config.Config) error {
				for i, t := range cfg.Tokens {
					if t.ChainID == cmd.String("chain-id") && strings.EqualFold(t.Address, cmd.String("address")) {
						cfg.Tokens = append(cfg.Tokens[:i], cfg.Tokens[i+1:]...)
						return nil
					}
				}
				return errors.New("token not configured")
			})
		}},
	}}
}
