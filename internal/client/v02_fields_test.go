package client

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"testing"
)

// v0.2 added the expected_answer_format enum + the three alt-media URL slots
// (video_link, video_link_2, answer_media) and their start/end timestamps.
// These tests cover the wire-level mapping in both JSON and multipart branches.

func TestCreateQuestionSendsExpectedAnswerFormatInJSON(t *testing.T) {
	c, srv := newTestClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body := readJSON(t, r)
		q := body["question"].(map[string]any)
		if q["expected_answer_format"] != "video" {
			t.Errorf("expected_answer_format = %v, want %q", q["expected_answer_format"], "video")
		}
		w.WriteHeader(201)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"question":{"id":1,"name":"D1","suit":{"name":"Demonstrate"}}}`))
	}))
	defer srv.Close()

	_, err := c.CreateQuestion(context.Background(), 42, QuestionAttrs{
		SuitName:                "Demonstrate",
		Text:                    "Record a demo",
		Answer:                  "...",
		ExpectedAnswerFormat:    "video",
		HasExpectedAnswerFormat: true,
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestUpdateQuestionSendsAllAltMediaFields(t *testing.T) {
	c, srv := newTestClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body := readJSON(t, r)
		q := body["question"].(map[string]any)

		checks := map[string]string{
			"video_link":         "https://youtube.com/watch?v=abc",
			"video_link_start":   "0:30",
			"video_link_end":     "1:45",
			"video_link_2":       "https://youtube.com/watch?v=xyz",
			"video_link_2_start": "0:00",
			"video_link_2_end":   "0:15",
			"answer_media":       "https://youtube.com/watch?v=def",
			"answer_media_start": "10",
			"answer_media_end":   "30",
		}
		for k, want := range checks {
			if got := q[k]; got != want {
				t.Errorf("%s = %v, want %q", k, got, want)
			}
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"question":{"id":1,"name":"M1","suit":{"name":"Mystery"}}}`))
	}))
	defer srv.Close()

	_, err := c.UpdateQuestion(context.Background(), 42, 1, QuestionAttrs{
		VideoLink: "https://youtube.com/watch?v=abc", HasVideoLink: true,
		VideoLinkStart: "0:30", HasVideoLinkStart: true,
		VideoLinkEnd: "1:45", HasVideoLinkEnd: true,
		VideoLink2: "https://youtube.com/watch?v=xyz", HasVideoLink2: true,
		VideoLink2Start: "0:00", HasVideoLink2Start: true,
		VideoLink2End: "0:15", HasVideoLink2End: true,
		AnswerMedia: "https://youtube.com/watch?v=def", HasAnswerMedia: true,
		AnswerMediaStart: "10", HasAnswerMediaStart: true,
		AnswerMediaEnd: "30", HasAnswerMediaEnd: true,
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestUpdateQuestionEmptyVideoLinkSendsEmptyString(t *testing.T) {
	// Has* without a value should send the empty string — letting the user
	// clear a video link by passing --video-link "".
	c, srv := newTestClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body := readJSON(t, r)
		q := body["question"].(map[string]any)
		got, ok := q["video_link"]
		if !ok {
			t.Errorf("video_link key absent — should be present (as empty string)")
		}
		if got != "" {
			t.Errorf("video_link = %v, want empty string", got)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"question":{"id":1,"name":"M1","suit":{"name":"Mystery"}}}`))
	}))
	defer srv.Close()

	_, err := c.UpdateQuestion(context.Background(), 42, 1, QuestionAttrs{
		HasVideoLink: true,
		VideoLink:    "",
	})
	if err != nil {
		t.Fatal(err)
	}
}

func TestUpdateQuestionOmitsFieldsNotMarkedHas(t *testing.T) {
	// If Has* is false, the key must NOT appear in the wire body.
	c, srv := newTestClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body := readJSON(t, r)
		q := body["question"].(map[string]any)
		for _, k := range []string{
			"expected_answer_format", "video_link", "video_link_start", "video_link_end",
			"video_link_2", "video_link_2_start", "video_link_2_end",
			"answer_media", "answer_media_start", "answer_media_end",
		} {
			if _, present := q[k]; present {
				t.Errorf("key %q should be absent when Has* is false; got %v", k, q[k])
			}
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"question":{"id":1,"name":"M1","suit":{"name":"Mystery"}}}`))
	}))
	defer srv.Close()

	// Set text only — all Has* booleans default false.
	_, err := c.UpdateQuestion(context.Background(), 42, 1, QuestionAttrs{Text: "just a text change"})
	if err != nil {
		t.Fatal(err)
	}
}

func TestMultipartCarriesV02FieldsWhenPictureAttached(t *testing.T) {
	dir := t.TempDir()
	picture := filepath.Join(dir, "p.png")
	if err := os.WriteFile(picture, []byte("\x89PNG"), 0o644); err != nil {
		t.Fatal(err)
	}

	var receivedBody []byte
	var receivedCT string
	c, srv := newTestClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedCT = r.Header.Get("Content-Type")
		receivedBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(201)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"question":{"id":1,"name":"M1","suit":{"name":"Mystery"}}}`))
	}))
	defer srv.Close()

	_, err := c.CreateQuestion(context.Background(), 42, QuestionAttrs{
		SuitName: "Mystery", Text: "Look at this", Answer: "...",
		Picture:                 picture,
		ExpectedAnswerFormat:    "voice",
		HasExpectedAnswerFormat: true,
		VideoLink:               "https://youtube.com/watch?v=abc",
		HasVideoLink:            true,
		AnswerMediaStart:        "5",
		HasAnswerMediaStart:     true,
	})
	if err != nil {
		t.Fatal(err)
	}

	_, params, err := mime.ParseMediaType(receivedCT)
	if err != nil {
		t.Fatal(err)
	}
	mr := multipart.NewReader(bytes.NewReader(receivedBody), params["boundary"])
	parts := map[string]string{}
	for {
		part, err := mr.NextPart()
		if err != nil {
			break
		}
		b, _ := io.ReadAll(part)
		parts[part.FormName()] = string(b)
	}

	checks := map[string]string{
		"question[expected_answer_format]": "voice",
		"question[video_link]":             "https://youtube.com/watch?v=abc",
		"question[answer_media_start]":     "5",
	}
	for k, want := range checks {
		if got := parts[k]; got != want {
			t.Errorf("multipart field %q = %q, want %q", k, got, want)
		}
	}
	// Confirm fields we DIDN'T set are absent.
	for _, k := range []string{
		"question[video_link_start]", "question[video_link_2]", "question[answer_media]",
	} {
		if _, present := parts[k]; present {
			t.Errorf("multipart field %q should be absent (Has* was false)", k)
		}
	}
}

func TestValidAnswerFormatsContainsExactlyTheEnum(t *testing.T) {
	want := []string{"general", "image", "video", "voice"}
	if len(ValidAnswerFormats) != len(want) {
		t.Fatalf("ValidAnswerFormats = %v, want %v", ValidAnswerFormats, want)
	}
	for i, v := range want {
		if ValidAnswerFormats[i] != v {
			t.Errorf("ValidAnswerFormats[%d] = %q, want %q", i, ValidAnswerFormats[i], v)
		}
	}
}

// Sanity: the existing ListQuestions JSON view returns expected_answer_format
// on each question; make sure pagination doesn't strip it. Uses the same
// pageSize=50 path as the production code.
func TestListQuestionsPreservesExpectedAnswerFormat(t *testing.T) {
	c, srv := newTestClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"deck_id": 42,
			"questions": []map[string]any{
				{"id": float64(1), "name": "M1", "expected_answer_format": "video"},
				{"id": float64(2), "name": "J1", "expected_answer_format": "general"},
			},
		})
	}))
	defer srv.Close()

	qs, err := c.ListQuestions(context.Background(), 42)
	if err != nil {
		t.Fatal(err)
	}
	if len(qs) != 2 {
		t.Fatalf("len = %d", len(qs))
	}
	first := qs[0].(map[string]any)
	if first["expected_answer_format"] != "video" {
		t.Errorf("first.expected_answer_format = %v, want video", first["expected_answer_format"])
	}
}

// Plumbing sanity: confirm that constructing QuestionAttrs without setting
// any Has* booleans produces a question JSON body with none of the v0.2
// keys (regression guard against accidentally inverting a default).
func TestQuestionJSONBodyOmitsUnsetV02Keys(t *testing.T) {
	body := questionJSONBody(QuestionAttrs{Text: "x"})
	for _, k := range []string{
		"expected_answer_format", "video_link", "video_link_start", "video_link_end",
		"video_link_2", "video_link_2_start", "video_link_2_end",
		"answer_media", "answer_media_start", "answer_media_end",
	} {
		if _, present := body[k]; present {
			t.Errorf("key %q present when Has* false: %v", k, body[k])
		}
	}
}
