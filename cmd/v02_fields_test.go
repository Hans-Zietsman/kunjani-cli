package cmd

import (
	"strings"
	"testing"
)

func TestActivityToAttrsParsesAnswerFormat(t *testing.T) {
	cases := []struct {
		in   map[string]any
		want string
	}{
		{map[string]any{"answer_format": "video"}, "video"},
		{map[string]any{"answer_format": "Video"}, "video"}, // case-insensitive
		{map[string]any{"expected_answer_format": "voice"}, "voice"},
		// expected_answer_format wins over answer_format because firstPresent
		// checks expected_answer_format first.
		{map[string]any{"expected_answer_format": "image", "answer_format": "general"}, "image"},
	}
	for i, c := range cases {
		attrs, err := activityToAttrs(c.in, t.TempDir())
		if err != nil {
			t.Fatalf("case %d: %v", i, err)
		}
		if !attrs.HasExpectedAnswerFormat {
			t.Errorf("case %d: HasExpectedAnswerFormat false", i)
		}
		if attrs.ExpectedAnswerFormat != c.want {
			t.Errorf("case %d: got %q want %q", i, attrs.ExpectedAnswerFormat, c.want)
		}
	}
}

func TestActivityToAttrsRejectsInvalidAnswerFormat(t *testing.T) {
	_, err := activityToAttrs(map[string]any{"answer_format": "movie"}, t.TempDir())
	if err == nil {
		t.Fatal("expected error for invalid answer_format")
	}
	if !strings.Contains(err.Error(), "movie") {
		t.Errorf("error doesn't mention bad value: %v", err)
	}
}

func TestActivityToAttrsParsesAltMediaURLs(t *testing.T) {
	a := map[string]any{
		"video_link":         "https://youtu.be/abc",
		"video_link_start":   "0:30",
		"video_link_end":     "1:45",
		"video_link_2":       "https://youtu.be/xyz",
		"video_link_2_start": "0:00",
		"video_link_2_end":   "0:15",
		"answer_media":       "https://youtu.be/def",
		"answer_media_start": float64(10), // JSON number → string in attrs
		"answer_media_end":   float64(30),
	}
	attrs, err := activityToAttrs(a, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	checks := []struct {
		name, want string
		has        bool
		got        string
	}{
		{"VideoLink", "https://youtu.be/abc", attrs.HasVideoLink, attrs.VideoLink},
		{"VideoLinkStart", "0:30", attrs.HasVideoLinkStart, attrs.VideoLinkStart},
		{"VideoLinkEnd", "1:45", attrs.HasVideoLinkEnd, attrs.VideoLinkEnd},
		{"VideoLink2", "https://youtu.be/xyz", attrs.HasVideoLink2, attrs.VideoLink2},
		{"VideoLink2Start", "0:00", attrs.HasVideoLink2Start, attrs.VideoLink2Start},
		{"VideoLink2End", "0:15", attrs.HasVideoLink2End, attrs.VideoLink2End},
		{"AnswerMedia", "https://youtu.be/def", attrs.HasAnswerMedia, attrs.AnswerMedia},
		{"AnswerMediaStart", "10", attrs.HasAnswerMediaStart, attrs.AnswerMediaStart},
		{"AnswerMediaEnd", "30", attrs.HasAnswerMediaEnd, attrs.AnswerMediaEnd},
	}
	for _, c := range checks {
		if !c.has {
			t.Errorf("%s: Has* false (expected true)", c.name)
		}
		if c.got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, c.got, c.want)
		}
	}
}

func TestActivityToAttrsLeavesAltMediaFieldsAloneWhenAbsent(t *testing.T) {
	attrs, err := activityToAttrs(map[string]any{"suit": "Jolt", "text": "x", "answer": "y"}, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if attrs.HasVideoLink || attrs.HasAnswerMedia || attrs.HasExpectedAnswerFormat {
		t.Errorf("Has* should be false when keys absent: vl=%v am=%v eaf=%v",
			attrs.HasVideoLink, attrs.HasAnswerMedia, attrs.HasExpectedAnswerFormat)
	}
}

func TestNormalizeAnswerFormatNormalizesCase(t *testing.T) {
	cases := map[string]string{
		"video":   "video",
		"VIDEO":   "video",
		" Voice ": "voice",
		"":        "",
	}
	for in, want := range cases {
		got, err := normalizeAnswerFormat(in)
		if err != nil {
			t.Errorf("normalizeAnswerFormat(%q): %v", in, err)
			continue
		}
		if got != want {
			t.Errorf("normalizeAnswerFormat(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestNormalizeAnswerFormatRejectsInvalid(t *testing.T) {
	_, err := normalizeAnswerFormat("movie")
	if err == nil {
		t.Fatal("expected error for invalid value")
	}
}
