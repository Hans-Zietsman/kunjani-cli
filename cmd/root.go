package cmd

import (
	"context"
	"errors"
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/Hans-Zietsman/kunjani-cli/internal/client"
	"github.com/Hans-Zietsman/kunjani-cli/internal/config"
)

type globalFlags struct {
	JSON bool
	Host string
}

var gf globalFlags

func NewRootCmd(version string) *cobra.Command {
	root := &cobra.Command{
		Use:           "kunjani",
		Short:         "Kunjani API CLI",
		Version:       version,
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.PersistentFlags().BoolVar(&gf.JSON, "json", false, "Emit JSON instead of human-readable output")
	root.PersistentFlags().StringVar(&gf.Host, "host", "", "Override the configured API host")

	root.AddCommand(newAuthCmd(version))
	root.AddCommand(newWhoamiCmd(version))
	root.AddCommand(newListDecksCmd(version))
	root.AddCommand(newCreateDeckCmd(version))
	root.AddCommand(newAddQuestionCmd(version))
	root.AddCommand(newUpdateQuestionCmd(version))
	root.AddCommand(newBulkAddCmd(version))
	root.AddCommand(newListOutcomesCmd(version))
	root.AddCommand(newAddOutcomeCmd(version))
	root.AddCommand(newUpdateOutcomeCmd(version))
	root.AddCommand(newDeleteOutcomeCmd(version))
	return root
}

type BulkAddError struct{}

func (*BulkAddError) Error() string { return "bulk-add: one or more activities failed" }

func ExitCode(err error) int {
	if err == nil {
		return 0
	}
	var ae *client.AuthError
	var nf *client.NotFoundError
	var ve *client.ValidationError
	var ge *client.GenericError
	var ba *BulkAddError

	switch {
	case errors.As(err, &ae):
		fmt.Fprintf(os.Stderr, "Authentication error: %s\n", ae.Msg)
		return 2
	case errors.As(err, &nf):
		fmt.Fprintf(os.Stderr, "Not found: %s\n", nf.Msg)
		return 3
	case errors.As(err, &ve):
		fmt.Fprintln(os.Stderr, "Validation failed:")
		for _, e := range ve.Errors {
			fmt.Fprintf(os.Stderr, "  - %s\n", e)
		}
		return 4
	case errors.As(err, &ba):
		return 1
	case errors.As(err, &ge):
		fmt.Fprintf(os.Stderr, "Error: %s\n", ge.Msg)
		return 1
	default:
		fmt.Fprintf(os.Stderr, "Error: %s\n", err.Error())
		return 1
	}
}

func loadConfig() (*config.Config, error) {
	c, err := config.Load(config.DefaultPath())
	if err != nil {
		return nil, err
	}
	if gf.Host != "" {
		c.Host = gf.Host
	}
	return c, nil
}

func makeClient(version string) (*client.Client, *config.Config, error) {
	cfg, err := loadConfig()
	if err != nil {
		return nil, nil, err
	}
	if !cfg.Configured() {
		fmt.Fprintln(os.Stderr, "No API token configured. Run: kunjani auth --token knj_... [--host URL]")
		os.Exit(1)
	}
	return client.New(cfg.EffectiveHost(), cfg.Token, version), cfg, nil
}

func ctx() context.Context {
	return context.Background()
}
