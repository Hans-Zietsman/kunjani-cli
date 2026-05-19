package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/Hans-Zietsman/kunjani-cli/internal/client"
	"github.com/Hans-Zietsman/kunjani-cli/internal/output"
)

func newAddQuestionCmd(version string) *cobra.Command {
	var (
		deck            int
		suit            string
		text            string
		answer          string
		timeSeconds     int
		nameFlag        string
		assessmentNotes string
		picture         string
		facilitatorPic  string
		outcomes        string
	)
	cmd := &cobra.Command{
		Use:   "add-question",
		Short: "Create a question in a deck",
		RunE: func(cmd *cobra.Command, args []string) error {
			cli, _, err := makeClient(version)
			if err != nil {
				return err
			}
			pic, err := requireFile(picture, "--picture")
			if err != nil {
				return err
			}
			facPic, err := requireFile(facilitatorPic, "--facilitator-picture")
			if err != nil {
				return err
			}
			attrs := client.QuestionAttrs{
				SuitName:           suit,
				Text:               text,
				Answer:             answer,
				Name:               nameFlag,
				AssessmentNotes:    assessmentNotes,
				Picture:            pic,
				FacilitatorPicture: facPic,
			}
			if cmd.Flags().Changed("time") {
				t := timeSeconds
				attrs.TimeInSeconds = &t
			}
			if cmd.Flags().Changed("outcomes") {
				attrs.HasOutcomeDescriptions = true
				attrs.OutcomeDescriptions = splitOutcomes(outcomes)
			}
			q, err := cli.CreateQuestion(ctx(), deck, attrs)
			if err != nil {
				return err
			}
			if gf.JSON {
				return output.WriteJSON(os.Stdout, map[string]any{"question": q})
			}
			suitName := ""
			if s, ok := q["suit"].(map[string]any); ok {
				suitName = stringOf(s["name"])
			}
			fmt.Fprintf(os.Stdout, "Created question #%s %q (%s)\n", stringOf(q["id"]), stringOf(q["name"]), suitName)
			return nil
		},
	}
	cmd.Flags().IntVar(&deck, "deck", 0, "Deck ID")
	cmd.Flags().StringVar(&suit, "suit", "", "Suit name (Jolt, Mystery, Advance, Oops/Blaps, Explain, Demonstrate)")
	cmd.Flags().StringVar(&text, "text", "", "Question text")
	cmd.Flags().StringVar(&answer, "answer", "", "Answer")
	cmd.Flags().IntVar(&timeSeconds, "time", 0, "Time in seconds")
	cmd.Flags().StringVar(&nameFlag, "name", "", "Explicit question name")
	cmd.Flags().StringVar(&assessmentNotes, "assessment-notes", "", "Assessment notes")
	cmd.Flags().StringVar(&picture, "picture", "", "Path to question media")
	cmd.Flags().StringVar(&facilitatorPic, "facilitator-picture", "", "Path to facilitator media")
	cmd.Flags().StringVar(&outcomes, "outcomes", "", "Comma-separated outcome descriptions")
	cmd.MarkFlagRequired("deck")
	cmd.MarkFlagRequired("suit")
	cmd.MarkFlagRequired("text")
	cmd.MarkFlagRequired("answer")
	return cmd
}

func newUpdateQuestionCmd(version string) *cobra.Command {
	var (
		deck              int
		question          int
		suit              string
		text              string
		answer            string
		timeSeconds       int
		nameFlag          string
		assessmentNotes   string
		picture           string
		facilitatorPic    string
		removePic         bool
		removeFacPic      bool
		outcomes          string
	)
	cmd := &cobra.Command{
		Use:   "update-question",
		Short: "Update fields on (or attach media to) an existing question",
		RunE: func(cmd *cobra.Command, args []string) error {
			cli, _, err := makeClient(version)
			if err != nil {
				return err
			}
			pic, err := requireFile(picture, "--picture")
			if err != nil {
				return err
			}
			facPic, err := requireFile(facilitatorPic, "--facilitator-picture")
			if err != nil {
				return err
			}
			attrs := client.QuestionAttrs{
				SuitName:                 suit,
				Text:                     text,
				Answer:                   answer,
				Name:                     nameFlag,
				AssessmentNotes:          assessmentNotes,
				Picture:                  pic,
				FacilitatorPicture:       facPic,
				RemovePicture:            removePic,
				RemoveFacilitatorPicture: removeFacPic,
			}
			if cmd.Flags().Changed("time") {
				t := timeSeconds
				attrs.TimeInSeconds = &t
			}
			if cmd.Flags().Changed("outcomes") {
				attrs.HasOutcomeDescriptions = true
				attrs.OutcomeDescriptions = splitOutcomes(outcomes)
			}
			q, err := cli.UpdateQuestion(ctx(), deck, question, attrs)
			if err != nil {
				return err
			}
			if gf.JSON {
				return output.WriteJSON(os.Stdout, map[string]any{"question": q})
			}
			fmt.Fprintf(os.Stdout, "Updated question #%s %q\n", stringOf(q["id"]), stringOf(q["name"]))
			return nil
		},
	}
	cmd.Flags().IntVar(&deck, "deck", 0, "Deck ID")
	cmd.Flags().IntVar(&question, "question", 0, "Question ID")
	cmd.Flags().StringVar(&suit, "suit", "", "Reassign to a different suit")
	cmd.Flags().StringVar(&text, "text", "", "Question text")
	cmd.Flags().StringVar(&answer, "answer", "", "Answer")
	cmd.Flags().IntVar(&timeSeconds, "time", 0, "Time in seconds")
	cmd.Flags().StringVar(&nameFlag, "name", "", "Explicit question name")
	cmd.Flags().StringVar(&assessmentNotes, "assessment-notes", "", "Assessment notes")
	cmd.Flags().StringVar(&picture, "picture", "", "Path to question media")
	cmd.Flags().StringVar(&facilitatorPic, "facilitator-picture", "", "Path to facilitator media")
	cmd.Flags().BoolVar(&removePic, "remove-picture", false, "Remove existing picture")
	cmd.Flags().BoolVar(&removeFacPic, "remove-facilitator-picture", false, "Remove existing facilitator picture")
	cmd.Flags().StringVar(&outcomes, "outcomes", "", `Comma-separated outcome descriptions; pass "" to clear all outcomes`)
	cmd.MarkFlagRequired("deck")
	cmd.MarkFlagRequired("question")
	return cmd
}

func newBulkAddCmd(version string) *cobra.Command {
	var (
		deck     int
		filePath string
	)
	cmd := &cobra.Command{
		Use:   "bulk-add",
		Short: "Create multiple questions (optionally pre-seed outcomes) from a JSON file",
		RunE: func(cmd *cobra.Command, args []string) error {
			cli, _, err := makeClient(version)
			if err != nil {
				return err
			}
			expanded := expandPath(filePath)
			raw, err := os.ReadFile(expanded)
			if err != nil {
				return err
			}
			var generic any
			if err := json.Unmarshal(raw, &generic); err != nil {
				return fmt.Errorf("parse %s: %w", expanded, err)
			}

			var preSeed []string
			var activities []map[string]any

			switch v := generic.(type) {
			case []any:
				for _, item := range v {
					if m, ok := item.(map[string]any); ok {
						activities = append(activities, m)
					}
				}
			case map[string]any:
				if pa, ok := v["activities"].([]any); ok {
					for _, item := range pa {
						if m, ok := item.(map[string]any); ok {
							activities = append(activities, m)
						}
					}
				}
				if po, ok := v["outcomes"].([]any); ok {
					for _, o := range po {
						if s, ok := o.(string); ok {
							preSeed = append(preSeed, s)
						}
					}
				}
			}
			if len(activities) == 0 {
				return &client.GenericError{Msg: `No activities found in file (expected an "activities" array).`}
			}

			baseDir := filepath.Dir(expanded)

			var preSeeded []map[string]any
			if len(preSeed) > 0 {
				existing, err := cli.ListOutcomes(ctx(), deck)
				if err != nil {
					return err
				}
				existingMap := map[string]bool{}
				for _, o := range existing {
					om := o.(map[string]any)
					existingMap[strings.ToLower(strings.TrimSpace(stringOf(om["description"])))] = true
				}
				for _, desc := range preSeed {
					d := strings.TrimSpace(desc)
					if d == "" {
						continue
					}
					if existingMap[strings.ToLower(d)] {
						if !gf.JSON {
							fmt.Fprintf(os.Stdout, "  outcome (existing) — %s\n", d)
						}
						continue
					}
					out, err := cli.CreateOutcome(ctx(), deck, client.OutcomeCreate{Description: d})
					if err != nil {
						if !gf.JSON {
							fmt.Fprintf(os.Stdout, "  outcome %q FAILED: %s\n", d, err.Error())
						}
						continue
					}
					preSeeded = append(preSeeded, out)
					if !gf.JSON {
						fmt.Fprintf(os.Stdout, "  outcome — created %q (#%s)\n", stringOf(out["description"]), stringOf(out["id"]))
					}
				}
			}

			var created []map[string]any
			var failures []map[string]any
			for i, a := range activities {
				attrs, err := activityToAttrs(a, baseDir)
				if err != nil {
					failures = append(failures, map[string]any{
						"index":    i,
						"error":    err.Error(),
						"activity": a,
					})
					if !gf.JSON {
						fmt.Fprintf(os.Stdout, "  [%d/%d] FAILED: %s\n", i+1, len(activities), err.Error())
					}
					continue
				}
				q, err := cli.CreateQuestion(ctx(), deck, attrs)
				if err != nil {
					failures = append(failures, map[string]any{
						"index":    i,
						"error":    err.Error(),
						"activity": a,
					})
					if !gf.JSON {
						fmt.Fprintf(os.Stdout, "  [%d/%d] FAILED: %s\n", i+1, len(activities), err.Error())
					}
					continue
				}
				created = append(created, q)
				if !gf.JSON {
					suitName := ""
					if s, ok := q["suit"].(map[string]any); ok {
						suitName = stringOf(s["name"])
					}
					fmt.Fprintf(os.Stdout, "  [%d/%d] %s — created %s (#%s)\n", i+1, len(activities), suitName, stringOf(q["name"]), stringOf(q["id"]))
				}
			}

			if gf.JSON {
				if err := output.WriteJSON(os.Stdout, map[string]any{
					"outcomes": preSeeded,
					"created":  created,
					"failed":   failures,
				}); err != nil {
					return err
				}
			} else {
				fmt.Fprintf(os.Stdout, "\n%d outcomes seeded, %d questions created, %d failed.\n",
					len(preSeeded), len(created), len(failures))
			}

			if len(failures) > 0 {
				return &BulkAddError{}
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&deck, "deck", 0, "Deck ID")
	cmd.Flags().StringVar(&filePath, "file", "", `Path to JSON file with {"outcomes": [...], "activities": [...]}`)
	cmd.MarkFlagRequired("deck")
	cmd.MarkFlagRequired("file")
	return cmd
}

func activityToAttrs(a map[string]any, baseDir string) (client.QuestionAttrs, error) {
	attrs := client.QuestionAttrs{}
	attrs.SuitName = firstNonEmpty(stringOf(a["suit"]), stringOf(a["suit_name"]))
	attrs.Text = stringOf(a["text"])
	attrs.Answer = stringOf(a["answer"])
	attrs.Name = stringOf(a["name"])
	attrs.AssessmentNotes = stringOf(a["assessment_notes"])

	if v, ok := numericField(a, "time_in_seconds", "time"); ok {
		t := v
		attrs.TimeInSeconds = &t
	}

	if pic, ok := a["picture"]; ok {
		if s, ok := pic.(string); ok && strings.TrimSpace(s) != "" {
			p, err := resolvePath(s, baseDir)
			if err != nil {
				return attrs, err
			}
			attrs.Picture = p
		}
	}
	if pic, ok := a["facilitator_picture"]; ok {
		if s, ok := pic.(string); ok && strings.TrimSpace(s) != "" {
			p, err := resolvePath(s, baseDir)
			if err != nil {
				return attrs, err
			}
			attrs.FacilitatorPicture = p
		}
	}

	if v, ok := a["outcomes"]; ok && v != nil {
		var descs []string
		switch arr := v.(type) {
		case []any:
			for _, item := range arr {
				s := strings.TrimSpace(stringOf(item))
				if s != "" {
					descs = append(descs, s)
				}
			}
		case string:
			s := strings.TrimSpace(arr)
			if s != "" {
				descs = append(descs, s)
			}
		}
		if descs == nil {
			descs = []string{}
		}
		attrs.HasOutcomeDescriptions = true
		attrs.OutcomeDescriptions = descs
	}

	return attrs, nil
}

func numericField(a map[string]any, keys ...string) (int, bool) {
	for _, k := range keys {
		v, ok := a[k]
		if !ok || v == nil {
			continue
		}
		switch n := v.(type) {
		case float64:
			return int(n), true
		case int:
			return n, true
		}
	}
	return 0, false
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if v != "" {
			return v
		}
	}
	return ""
}

func splitOutcomes(s string) []string {
	if strings.TrimSpace(s) == "" {
		return []string{}
	}
	out := []string{}
	for _, p := range strings.Split(s, ",") {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

func requireFile(path, flagName string) (string, error) {
	if path == "" {
		return "", nil
	}
	expanded := expandPath(path)
	info, err := os.Stat(expanded)
	if err != nil || info.IsDir() {
		return "", &client.GenericError{Msg: fmt.Sprintf("%s %s: file not found", flagName, path)}
	}
	return expanded, nil
}

func resolvePath(value, baseDir string) (string, error) {
	if strings.TrimSpace(value) == "" {
		return "", nil
	}
	var candidate string
	if filepath.IsAbs(value) {
		candidate = value
	} else {
		candidate = filepath.Join(baseDir, value)
	}
	info, err := os.Stat(candidate)
	if err != nil || info.IsDir() {
		return "", &client.GenericError{Msg: fmt.Sprintf("picture/facilitator_picture not found: %s", candidate)}
	}
	return candidate, nil
}

func expandPath(path string) string {
	if strings.HasPrefix(path, "~/") {
		if home, err := os.UserHomeDir(); err == nil {
			return filepath.Join(home, path[2:])
		}
	}
	abs, err := filepath.Abs(path)
	if err != nil {
		return path
	}
	return abs
}

func newListQuestionsCmd(version string) *cobra.Command {
	var (
		deck int
		suit string
	)
	cmd := &cobra.Command{
		Use:   "list-questions",
		Short: "List questions on a deck (paged internally, returns one flat slice)",
		RunE: func(cmd *cobra.Command, args []string) error {
			cli, _, err := makeClient(version)
			if err != nil {
				return err
			}
			qs, err := cli.ListQuestions(ctx(), deck)
			if err != nil {
				return err
			}
			if suit != "" {
				want := strings.ToLower(strings.TrimSpace(suit))
				filtered := make([]any, 0, len(qs))
				for _, q := range qs {
					m, _ := q.(map[string]any)
					if m == nil {
						continue
					}
					s, _ := m["suit"].(map[string]any)
					if s == nil {
						continue
					}
					if strings.ToLower(stringOf(s["name"])) == want {
						filtered = append(filtered, q)
					}
				}
				qs = filtered
			}

			if gf.JSON {
				return output.WriteJSON(os.Stdout, map[string]any{"questions": qs})
			}
			if len(qs) == 0 {
				fmt.Fprintln(os.Stdout, "No questions on this deck.")
				return nil
			}
			rows := [][]string{{"ID", "NAME", "SUIT", "TIME", "MEDIA"}}
			for _, q := range qs {
				m := q.(map[string]any)
				suitName := ""
				if s, ok := m["suit"].(map[string]any); ok {
					suitName = stringOf(s["name"])
				}
				timeCell := ""
				if t := stringOf(m["time_in_seconds"]); t != "" {
					timeCell = t + "s"
				}
				media := "-"
				if url, ok := m["picture_url"].(string); ok && url != "" {
					media = pictureURLBasename(url)
				}
				rows = append(rows, []string{
					stringOf(m["id"]),
					stringOf(m["name"]),
					suitName,
					timeCell,
					media,
				})
			}
			output.PrintTable(os.Stdout, rows)
			return nil
		},
	}
	cmd.Flags().IntVar(&deck, "deck", 0, "Deck ID")
	cmd.Flags().StringVar(&suit, "suit", "", "Optional case-insensitive suit filter (Jolt, Mystery, Advance, Oops, Explain, Demonstrate)")
	cmd.MarkFlagRequired("deck")
	return cmd
}

func newGetQuestionCmd(version string) *cobra.Command {
	var (
		deck     int
		question int
	)
	cmd := &cobra.Command{
		Use:   "get-question",
		Short: "Show the full activity for one question on a deck",
		RunE: func(cmd *cobra.Command, args []string) error {
			cli, _, err := makeClient(version)
			if err != nil {
				return err
			}
			q, err := cli.GetQuestion(ctx(), deck, question)
			if err != nil {
				return err
			}
			if gf.JSON {
				return output.WriteJSON(os.Stdout, map[string]any{"question": q})
			}

			suitName := ""
			if s, ok := q["suit"].(map[string]any); ok {
				suitName = stringOf(s["name"])
			}
			fmt.Fprintf(os.Stdout, "Question #%s %q (%s)\n", stringOf(q["id"]), stringOf(q["name"]), suitName)
			if t := stringOf(q["time_in_seconds"]); t != "" {
				fmt.Fprintf(os.Stdout, "Time: %ss\n", t)
			}
			if url, ok := q["picture_url"].(string); ok && url != "" {
				fmt.Fprintf(os.Stdout, "Media: %s\n", url)
			}
			if outs, ok := q["outcomes"].([]any); ok && len(outs) > 0 {
				descs := make([]string, 0, len(outs))
				for _, o := range outs {
					if m, ok := o.(map[string]any); ok {
						descs = append(descs, stringOf(m["description"]))
					}
				}
				if len(descs) > 0 {
					fmt.Fprintf(os.Stdout, "Outcomes: %s\n", strings.Join(descs, "; "))
				}
			}
			if txt := stringOf(q["text"]); txt != "" {
				fmt.Fprintf(os.Stdout, "\nText:\n%s\n", txt)
			}
			if ans := stringOf(q["answer"]); ans != "" {
				fmt.Fprintf(os.Stdout, "\nAnswer:\n%s\n", ans)
			}
			if notes := stringOf(q["assessment_notes"]); notes != "" {
				fmt.Fprintf(os.Stdout, "\nAssessment notes:\n%s\n", notes)
			}
			return nil
		},
	}
	cmd.Flags().IntVar(&deck, "deck", 0, "Deck ID")
	cmd.Flags().IntVar(&question, "question", 0, "Question ID")
	cmd.MarkFlagRequired("deck")
	cmd.MarkFlagRequired("question")
	return cmd
}

func pictureURLBasename(url string) string {
	if i := strings.Index(url, "?"); i >= 0 {
		url = url[:i]
	}
	if i := strings.LastIndex(url, "/"); i >= 0 {
		return url[i+1:]
	}
	return url
}
