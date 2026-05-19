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

func newListOutcomesCmd(version string) *cobra.Command {
	var deck int
	cmd := &cobra.Command{
		Use:   "list-outcomes",
		Short: "List the outcomes on a deck (direct + transitive via questions)",
		RunE: func(cmd *cobra.Command, args []string) error {
			cli, _, err := makeClient(version)
			if err != nil {
				return err
			}
			outcomes, err := cli.ListOutcomes(ctx(), deck)
			if err != nil {
				return err
			}
			if gf.JSON {
				return output.WriteJSON(os.Stdout, map[string]any{"outcomes": outcomes})
			}
			if len(outcomes) == 0 {
				fmt.Fprintln(os.Stdout, "No outcomes on this deck.")
				return nil
			}
			rows := [][]string{{"ID", "KIND", "DESCRIPTION", "QUESTIONS"}}
			for _, o := range outcomes {
				m := o.(map[string]any)
				rows = append(rows, []string{
					stringOf(m["id"]),
					stringOf(m["kind"]),
					stringOf(m["description"]),
					joinIDs(m["question_ids"]),
				})
			}
			output.PrintTable(os.Stdout, rows)
			return nil
		},
	}
	cmd.Flags().IntVar(&deck, "deck", 0, "Deck ID")
	cmd.MarkFlagRequired("deck")
	return cmd
}

func newAddOutcomeCmd(version string) *cobra.Command {
	var (
		deck        int
		description string
		questions   string
	)
	cmd := &cobra.Command{
		Use:   "add-outcome",
		Short: "Create an outcome on a deck (optionally attach to questions)",
		RunE: func(cmd *cobra.Command, args []string) error {
			cli, _, err := makeClient(version)
			if err != nil {
				return err
			}
			attrs := client.OutcomeCreate{Description: description}
			if cmd.Flags().Changed("questions") {
				attrs.HasQuestionIDs = true
				attrs.QuestionIDs = parseIDList(questions)
			}
			out, err := cli.CreateOutcome(ctx(), deck, attrs)
			if err != nil {
				return err
			}
			if gf.JSON {
				return output.WriteJSON(os.Stdout, map[string]any{"outcome": out})
			}
			fmt.Fprintf(os.Stdout, "Created outcome #%s %q\n", stringOf(out["id"]), stringOf(out["description"]))
			return nil
		},
	}
	cmd.Flags().IntVar(&deck, "deck", 0, "Deck ID")
	cmd.Flags().StringVar(&description, "description", "", "Outcome description")
	cmd.Flags().StringVar(&questions, "questions", "", "Comma-separated question IDs to attach this outcome to")
	cmd.MarkFlagRequired("deck")
	cmd.MarkFlagRequired("description")
	return cmd
}

func newUpdateOutcomeCmd(version string) *cobra.Command {
	var (
		deck        int
		outcomeID   int
		description string
		questions   string
	)
	cmd := &cobra.Command{
		Use:   "update-outcome",
		Short: "Update an outcome on a deck",
		RunE: func(cmd *cobra.Command, args []string) error {
			cli, _, err := makeClient(version)
			if err != nil {
				return err
			}
			attrs := client.OutcomeUpdate{}
			if cmd.Flags().Changed("description") {
				attrs.HasDescription = true
				attrs.Description = description
			}
			if cmd.Flags().Changed("questions") {
				attrs.HasQuestionIDs = true
				attrs.QuestionIDs = parseIDList(questions)
			}
			if !attrs.HasDescription && !attrs.HasQuestionIDs {
				return &client.GenericError{Msg: "Nothing to update. Pass --description and/or --questions."}
			}
			out, err := cli.UpdateOutcome(ctx(), deck, outcomeID, attrs)
			if err != nil {
				return err
			}
			if gf.JSON {
				return output.WriteJSON(os.Stdout, map[string]any{"outcome": out})
			}
			fmt.Fprintf(os.Stdout, "Updated outcome #%s %q\n", stringOf(out["id"]), stringOf(out["description"]))
			return nil
		},
	}
	cmd.Flags().IntVar(&deck, "deck", 0, "Deck ID")
	cmd.Flags().IntVar(&outcomeID, "outcome", 0, "Outcome ID")
	cmd.Flags().StringVar(&description, "description", "", "New description")
	cmd.Flags().StringVar(&questions, "questions", "", `Comma-separated question IDs (replaces existing); pass "" to clear`)
	cmd.MarkFlagRequired("deck")
	cmd.MarkFlagRequired("outcome")
	return cmd
}

func newDeleteOutcomeCmd(version string) *cobra.Command {
	var (
		deck      int
		outcomeID int
	)
	cmd := &cobra.Command{
		Use:   "delete-outcome",
		Short: "Delete an outcome from a deck",
		RunE: func(cmd *cobra.Command, args []string) error {
			cli, _, err := makeClient(version)
			if err != nil {
				return err
			}
			if err := cli.DeleteOutcome(ctx(), deck, outcomeID); err != nil {
				return err
			}
			if gf.JSON {
				return output.WriteJSON(os.Stdout, map[string]any{"deleted": outcomeID})
			}
			fmt.Fprintf(os.Stdout, "Deleted outcome #%d\n", outcomeID)
			return nil
		},
	}
	cmd.Flags().IntVar(&deck, "deck", 0, "Deck ID")
	cmd.Flags().IntVar(&outcomeID, "outcome", 0, "Outcome ID")
	cmd.MarkFlagRequired("deck")
	cmd.MarkFlagRequired("outcome")
	return cmd
}

func parseIDList(value string) []int {
	if strings.TrimSpace(value) == "" {
		return []int{}
	}
	out := []int{}
	for _, p := range strings.Split(value, ",") {
		n, err := strconv.Atoi(strings.TrimSpace(p))
		if err != nil || n == 0 {
			continue
		}
		out = append(out, n)
	}
	return out
}

func joinIDs(v any) string {
	arr, ok := v.([]any)
	if !ok {
		return ""
	}
	parts := make([]string, 0, len(arr))
	for _, x := range arr {
		parts = append(parts, stringOf(x))
	}
	return strings.Join(parts, ",")
}
