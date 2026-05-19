package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/Hans-Zietsman/kunjani-cli/internal/client"
	"github.com/Hans-Zietsman/kunjani-cli/internal/output"
)

func newListDecksCmd(version string) *cobra.Command {
	return &cobra.Command{
		Use:   "list-decks",
		Short: "List decks you can access",
		RunE: func(cmd *cobra.Command, args []string) error {
			cli, _, err := makeClient(version)
			if err != nil {
				return err
			}
			decks, err := cli.ListDecks(ctx())
			if err != nil {
				return err
			}
			if gf.JSON {
				return output.WriteJSON(os.Stdout, map[string]any{"decks": decks})
			}
			if len(decks) == 0 {
				fmt.Fprintln(os.Stdout, "No decks found.")
				return nil
			}
			rows := [][]string{{"ID", "NAME", "VISIBILITY", "CREATED"}}
			for _, d := range decks {
				m := d.(map[string]any)
				rows = append(rows, []string{
					stringOf(m["id"]),
					stringOf(m["name"]),
					stringOf(m["visibility"]),
					first10(stringOf(m["created_at"])),
				})
			}
			output.PrintTable(os.Stdout, rows)
			return nil
		},
	}
}

func newCreateDeckCmd(version string) *cobra.Command {
	var description, visibility, collaborations string
	cmd := &cobra.Command{
		Use:   "create-deck NAME",
		Short: "Create a new deck",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cli, _, err := makeClient(version)
			if err != nil {
				return err
			}
			attrs := client.DeckCreate{
				Name:           args[0],
				Visibility:     visibility,
				Collaborations: collaborations,
			}
			if cmd.Flags().Changed("description") {
				attrs.Description = description
			}
			deck, err := cli.CreateDeck(ctx(), attrs)
			if err != nil {
				return err
			}
			if gf.JSON {
				return output.WriteJSON(os.Stdout, map[string]any{"deck": deck})
			}
			fmt.Fprintf(os.Stdout, "Created deck #%s %q\n", stringOf(deck["id"]), stringOf(deck["name"]))
			return nil
		},
	}
	cmd.Flags().StringVar(&description, "description", "", "Description")
	cmd.Flags().StringVar(&visibility, "visibility", "Private", "Public | Private")
	cmd.Flags().StringVar(&collaborations, "collaborations", "No", "Yes | Organization | No")
	return cmd
}

func first10(s string) string {
	if len(s) <= 10 {
		return s
	}
	return s[:10]
}
