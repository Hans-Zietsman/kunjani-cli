package cmd

import (
	"reflect"
	"strings"
	"testing"
)

func TestParseIDListHappyPath(t *testing.T) {
	cases := []struct {
		in   string
		want []int
	}{
		{"1", []int{1}},
		{"1,2,3", []int{1, 2, 3}},
		{" 1 , 2 , 3 ", []int{1, 2, 3}},
		{"42,17,8", []int{42, 17, 8}}, // order preserved
	}
	for _, c := range cases {
		got, err := parseStrictIDList(c.in)
		if err != nil {
			t.Errorf("parseStrictIDList(%q) returned error: %v", c.in, err)
			continue
		}
		if !reflect.DeepEqual(got, c.want) {
			t.Errorf("parseStrictIDList(%q) = %v, want %v", c.in, got, c.want)
		}
	}
}

func TestParseIDListRejectsBadInput(t *testing.T) {
	cases := []struct {
		in       string
		errMatch string
	}{
		{"", "non-empty"},
		{"   ", "non-empty"},
		{"1,2,", "empty ID"},
		{"1,abc,3", "invalid ID"},
		{"1,-2,3", "invalid ID"},
		{"1,0,3", "invalid ID"}, // 0 is not a valid question ID
		{"1,2,2", "duplicate"},
	}
	for _, c := range cases {
		_, err := parseStrictIDList(c.in)
		if err == nil {
			t.Errorf("parseStrictIDList(%q) returned no error", c.in)
			continue
		}
		if !strings.Contains(err.Error(), c.errMatch) {
			t.Errorf("parseStrictIDList(%q) error = %q, want substring %q", c.in, err.Error(), c.errMatch)
		}
	}
}

func TestReorderQuestionsCmdHasRequiredFlags(t *testing.T) {
	cmd := newReorderQuestionsCmd("test")
	for _, name := range []string{"deck", "order"} {
		if cmd.Flag(name) == nil {
			t.Errorf("--%s flag missing on reorder-questions", name)
		}
	}
}

func TestGetDeckCmdHasDeckFlag(t *testing.T) {
	if newGetDeckCmd("test").Flag("deck") == nil {
		t.Error("--deck flag missing on get-deck")
	}
}

func TestUpdateDeckCmdHasAllFlags(t *testing.T) {
	cmd := newUpdateDeckCmd("test")
	for _, name := range []string{"deck", "name", "description", "visibility", "collaborations", "dice-option"} {
		if cmd.Flag(name) == nil {
			t.Errorf("--%s flag missing on update-deck", name)
		}
	}
}

func TestDeleteDeckCmdRequiresConfirm(t *testing.T) {
	cmd := newDeleteDeckCmd("test")
	if cmd.Flag("confirm") == nil {
		t.Error("--confirm flag missing on delete-deck")
	}
	if cmd.Flag("deck") == nil {
		t.Error("--deck flag missing on delete-deck")
	}
}

func TestDeleteQuestionCmdRequiresConfirm(t *testing.T) {
	cmd := newDeleteQuestionCmd("test")
	for _, name := range []string{"deck", "question", "confirm"} {
		if cmd.Flag(name) == nil {
			t.Errorf("--%s flag missing on delete-question", name)
		}
	}
}
