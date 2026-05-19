package cmd

import (
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/Hans-Zietsman/kunjani-cli/internal/client"
	"github.com/Hans-Zietsman/kunjani-cli/internal/config"
	"github.com/Hans-Zietsman/kunjani-cli/internal/output"
)

func newAuthCmd(version string) *cobra.Command {
	var token, host string
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Save an API token (verifies it against the server before saving)",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(config.DefaultPath())
			if err != nil {
				return err
			}
			if host != "" {
				cfg.Host = host
			} else if gf.Host != "" {
				cfg.Host = gf.Host
			}
			cfg.Token = token

			cli := client.New(cfg.EffectiveHost(), cfg.Token, version)
			me, err := cli.Me(ctx())
			if err != nil {
				var ae *client.AuthError
				if errors.As(err, &ae) {
					fmt.Fprintf(os.Stderr, "Authentication failed: %s\n", ae.Msg)
					os.Exit(1)
				}
				return err
			}

			if err := cfg.Save(); err != nil {
				return err
			}

			if gf.JSON {
				return output.WriteJSON(os.Stdout, me)
			}
			fmt.Fprintf(os.Stdout, "Token verified. Logged in as %s (role: %s). Saved to %s.\n",
				stringOf(me["email"]), stringOf(me["role"]), cfg.Path())
			return nil
		},
	}
	cmd.Flags().StringVar(&token, "token", "", "Your knj_... API token")
	cmd.Flags().StringVar(&host, "host", "", "Kunjani host (e.g. http://localhost:3000)")
	cmd.MarkFlagRequired("token")
	return cmd
}

func newWhoamiCmd(version string) *cobra.Command {
	return &cobra.Command{
		Use:   "whoami",
		Short: "Show the user the current token belongs to",
		RunE: func(cmd *cobra.Command, args []string) error {
			cli, cfg, err := makeClient(version)
			if err != nil {
				return err
			}
			me, err := cli.Me(ctx())
			if err != nil {
				return err
			}
			if gf.JSON {
				return output.WriteJSON(os.Stdout, me)
			}
			fmt.Fprintf(os.Stdout, "%s (id: %s, role: %s) on %s\n",
				stringOf(me["email"]), stringOf(me["id"]), stringOf(me["role"]), cfg.EffectiveHost())
			return nil
		},
	}
}

func stringOf(v any) string {
	if v == nil {
		return ""
	}
	switch x := v.(type) {
	case string:
		return x
	case float64:
		if x == float64(int64(x)) {
			return fmt.Sprintf("%d", int64(x))
		}
		return fmt.Sprintf("%v", x)
	default:
		return fmt.Sprintf("%v", x)
	}
}
