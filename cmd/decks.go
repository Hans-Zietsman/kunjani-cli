package cmd

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"github.com/spf13/cobra"

	"github.com/Hans-Zietsman/kunjani-cli/internal/client"
	"github.com/Hans-Zietsman/kunjani-cli/internal/output"
)

// parseStrictIDList turns "1,2,3" (allowing whitespace around commas) into
// [1, 2, 3]. Unlike cmd.parseIDList (in outcomes.go) which is lossy and
// silently drops invalid tokens, this errors on empty input, duplicates, or
// any non-positive-integer token. Use the strict variant whenever order
// matters or a typo would change semantics — e.g. reorder-questions, where
// a dropped ID would produce a wrong play order.
func parseStrictIDList(raw string) ([]int, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, fmt.Errorf("must be a non-empty comma-separated list of question IDs")
	}
	tokens := strings.Split(raw, ",")
	out := make([]int, 0, len(tokens))
	seen := map[int]bool{}
	for _, t := range tokens {
		s := strings.TrimSpace(t)
		if s == "" {
			return nil, fmt.Errorf("empty ID in list")
		}
		n, err := strconv.Atoi(s)
		if err != nil || n <= 0 {
			return nil, fmt.Errorf("invalid ID %q (must be a positive integer)", t)
		}
		if seen[n] {
			return nil, fmt.Errorf("duplicate ID %d", n)
		}
		seen[n] = true
		out = append(out, n)
	}
	return out, nil
}

func normalizeDiceOption(raw string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "loaded":
		return "loaded", nil
	case "random":
		return "random", nil
	default:
		return "", fmt.Errorf("--dice-option must be \"loaded\" or \"random\" (got %q)", raw)
	}
}

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
	var description, visibility, collaborations, diceOption string
	cmd := &cobra.Command{
		Use:   "create-deck NAME",
		Short: "Create a new deck",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dice, err := normalizeDiceOption(diceOption)
			if err != nil {
				return err
			}
			cli, _, err := makeClient(version)
			if err != nil {
				return err
			}
			attrs := client.DeckCreate{
				Name:           args[0],
				Visibility:     visibility,
				Collaborations: collaborations,
				DiceOption:     dice,
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
	cmd.Flags().StringVar(&diceOption, "dice-option", "loaded", "loaded | random — loaded plays activities in a set order; random picks from the rolled suit")
	return cmd
}

func first10(s string) string {
	if len(s) <= 10 {
		return s
	}
	return s[:10]
}

func newReorderQuestionsCmd(version string) *cobra.Command {
	var deck int
	var order string
	cmd := &cobra.Command{
		Use:   "reorder-questions",
		Short: "Set the play order of questions on a deck (loaded-mode decks)",
		Long: "Set the play order of questions on a deck. Pass --deck and --order\n" +
			"with a comma-separated list of question IDs in the order you want them\n" +
			"to play. The server validates that every ID belongs to the deck and\n" +
			"rejects the whole batch otherwise.\n\n" +
			"Only meaningful for loaded-mode decks: random-mode decks ignore the\n" +
			"saved order and pick a random activity from the rolled suit.",
		RunE: func(cmd *cobra.Command, args []string) error {
			if deck == 0 {
				return fmt.Errorf("--deck is required")
			}
			ids, err := parseStrictIDList(order)
			if err != nil {
				return fmt.Errorf("--order: %w", err)
			}
			cli, _, err := makeClient(version)
			if err != nil {
				return err
			}
			d, err := cli.ReorderQuestions(ctx(), deck, ids)
			if err != nil {
				return err
			}
			if gf.JSON {
				return output.WriteJSON(os.Stdout, map[string]any{"deck": d})
			}
			fmt.Fprintf(os.Stdout, "Reordered %d questions on deck #%d\n", len(ids), deck)
			return nil
		},
	}
	cmd.Flags().IntVar(&deck, "deck", 0, "Deck ID (required)")
	cmd.Flags().StringVar(&order, "order", "", "Comma-separated question IDs in desired play order, e.g. \"42,17,8\"")
	return cmd
}
