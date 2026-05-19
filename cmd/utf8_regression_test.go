package cmd

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
)

// Mirrors the regression in the Ruby CLI where bulk_add read the JSON file
// with the platform's default encoding (US-ASCII when LANG is unset), tripping
// on the first multi-byte sequence. Go's os.ReadFile + json.Unmarshal doesn't
// have that footgun, but if anyone ever sneaks in a strings.ToLower /
// fold-to-ASCII / strconv.QuoteToASCII on the bulk-add path, this catches it.
func TestActivityToAttrsPreservesUTF8FromAgentKFixture(t *testing.T) {
	raw, err := os.ReadFile("../internal/client/testdata/agent_k_utf8.json")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}

	var env struct {
		Outcomes   []string         `json:"outcomes"`
		Activities []map[string]any `json:"activities"`
	}
	if err := json.Unmarshal(raw, &env); err != nil {
		t.Fatalf("parse fixture: %v", err)
	}
	if len(env.Activities) < 2 {
		t.Fatalf("expected >=2 activities, got %d", len(env.Activities))
	}

	attrs0, err := activityToAttrs(env.Activities[0], t.TempDir())
	if err != nil {
		t.Fatalf("activityToAttrs[0]: %v", err)
	}

	wantInActivity0 := []struct {
		field, name, rune string
	}{
		{"Text", "lungs emoji", "🫁"},
		{"Text", "math-bold B", "𝐁"},
		{"Answer", "math-bold n (small)", "𝐧"},
		{"AssessmentNotes", "math-bold G", "𝐆"},
	}
	for _, w := range wantInActivity0 {
		var got string
		switch w.field {
		case "Text":
			got = attrs0.Text
		case "Answer":
			got = attrs0.Answer
		case "AssessmentNotes":
			got = attrs0.AssessmentNotes
		}
		if !strings.Contains(got, w.rune) {
			t.Errorf("activity[0].%s missing %s (%q).\ngot: %q", w.field, w.name, w.rune, got)
		}
	}

	if !attrs0.HasOutcomeDescriptions {
		t.Error("activity[0] should have outcome descriptions set")
	}
	if len(attrs0.OutcomeDescriptions) != 1 || !strings.Contains(attrs0.OutcomeDescriptions[0], "🎯") {
		t.Errorf("outcome_descriptions missing target emoji.\ngot: %v", attrs0.OutcomeDescriptions)
	}

	attrs1, err := activityToAttrs(env.Activities[1], t.TempDir())
	if err != nil {
		t.Fatalf("activityToAttrs[1]: %v", err)
	}
	if !strings.Contains(attrs1.Text, "⚡") {
		t.Errorf("activity[1].Text missing lightning bolt.\ngot: %q", attrs1.Text)
	}
	if attrs1.TimeInSeconds == nil || *attrs1.TimeInSeconds != 45 {
		t.Errorf("activity[1].TimeInSeconds = %v, want 45", attrs1.TimeInSeconds)
	}
}
