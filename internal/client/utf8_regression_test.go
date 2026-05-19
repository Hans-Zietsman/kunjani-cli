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
	"strings"
	"testing"
)

// Sentinel runes drawn from testdata/agent_k_utf8.json. The Ruby CLI failed
// here when LANG was unset because File.read defaulted to US-ASCII; Go's
// os.ReadFile + json.Unmarshal handles UTF-8 unconditionally, but the bytes
// still need to survive the multipart write path without being downgraded
// to binary or transcoded.
const (
	runeLungs      = "🫁" // U+1FAC1 -> F0 9F AB 81 (4-byte UTF-8)
	runeMathBoldB  = "𝐁" // U+1D401 -> F0 9D 90 81
	runeMathBoldn  = "𝐧" // U+1D427 -> F0 9D 90 A7
	runeMathBoldG  = "𝐆" // U+1D406 -> F0 9D 90 86
	runeMathBoldt  = "𝐭" // U+1D42D -> F0 9D 90 AD
	runeLightning  = "⚡" // U+26A1  -> E2 9A A1     (3-byte UTF-8)
	runeTarget     = "🎯" // U+1F3AF -> F0 9F 8E AF
)

type agentKEnvelope struct {
	Outcomes   []string         `json:"outcomes"`
	Activities []map[string]any `json:"activities"`
}

func readFixture(t *testing.T) (agentKEnvelope, []byte) {
	t.Helper()
	raw, err := os.ReadFile("testdata/agent_k_utf8.json")
	if err != nil {
		t.Fatalf("read fixture: %v", err)
	}
	for _, r := range []string{runeLungs, runeMathBoldB, runeMathBoldG, runeLightning, runeTarget, runeMathBoldt} {
		if !bytes.Contains(raw, []byte(r)) {
			t.Fatalf("fixture corrupted: missing rune %q in raw bytes", r)
		}
	}
	var env agentKEnvelope
	if err := json.Unmarshal(raw, &env); err != nil {
		t.Fatalf("parse fixture: %v", err)
	}
	return env, raw
}

func attrsFromFixtureActivity(a map[string]any) QuestionAttrs {
	attrs := QuestionAttrs{
		SuitName:        fixtureString(a, "suit"),
		Text:            fixtureString(a, "text"),
		Answer:          fixtureString(a, "answer"),
		AssessmentNotes: fixtureString(a, "assessment_notes"),
	}
	if tv, ok := a["time_in_seconds"].(float64); ok {
		n := int(tv)
		attrs.TimeInSeconds = &n
	}
	if outs, ok := a["outcomes"].([]any); ok {
		descs := make([]string, 0, len(outs))
		for _, o := range outs {
			if s, ok := o.(string); ok {
				descs = append(descs, s)
			}
		}
		attrs.HasOutcomeDescriptions = true
		attrs.OutcomeDescriptions = descs
	}
	return attrs
}

func fixtureString(m map[string]any, key string) string {
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

func TestAgentKFixtureRoundTripJSONPreservesUTF8(t *testing.T) {
	env, _ := readFixture(t)
	if len(env.Activities) < 2 {
		t.Fatalf("expected 2+ activities, got %d", len(env.Activities))
	}

	var received []byte
	c, srv := newTestClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
			t.Errorf("Content-Type = %q", r.Header.Get("Content-Type"))
		}
		received, _ = io.ReadAll(r.Body)
		w.WriteHeader(201)
		w.Write([]byte(`{"question":{"id":1,"name":"J1","suit":{"name":"Oops"}}}`))
	}))
	defer srv.Close()

	attrs := attrsFromFixtureActivity(env.Activities[0])
	if _, err := c.CreateQuestion(context.Background(), 42, attrs); err != nil {
		t.Fatalf("CreateQuestion: %v", err)
	}

	// Each multi-byte sentinel must appear in the wire bytes verbatim.
	for _, r := range []string{runeLungs, runeMathBoldB, runeMathBoldn, runeMathBoldG} {
		if !bytes.Contains(received, []byte(r)) {
			t.Errorf("wire body missing rune %q (bytes %x)", r, []byte(r))
		}
	}

	// And the bytes must still decode as a JSON envelope whose string fields
	// contain the same runes (i.e. nothing got base64'd or \u-escaped).
	var parsed struct {
		Question struct {
			SuitName        string   `json:"suit_name"`
			Text            string   `json:"text"`
			Answer          string   `json:"answer"`
			AssessmentNotes string   `json:"assessment_notes"`
			OutcomeDescs    []string `json:"outcome_descriptions"`
		} `json:"question"`
	}
	if err := json.Unmarshal(received, &parsed); err != nil {
		t.Fatalf("server-side parse: %v", err)
	}
	if !strings.Contains(parsed.Question.Text, runeLungs) {
		t.Errorf("parsed text missing lungs emoji.\ntext: %q", parsed.Question.Text)
	}
	if !strings.Contains(parsed.Question.Answer, runeMathBoldn) {
		t.Errorf("parsed answer missing math-bold n.\nanswer: %q", parsed.Question.Answer)
	}
	if !strings.Contains(parsed.Question.AssessmentNotes, runeMathBoldG) {
		t.Errorf("parsed assessment_notes missing math-bold G")
	}
	if len(parsed.Question.OutcomeDescs) == 0 || !strings.Contains(parsed.Question.OutcomeDescs[0], runeTarget) {
		t.Errorf("outcome_descriptions missing target emoji.\ngot: %v", parsed.Question.OutcomeDescs)
	}
}

func TestAgentKFixtureRoundTripMultipartPreservesUTF8(t *testing.T) {
	env, _ := readFixture(t)

	// Attaching any file flips us to the multipart branch — that's the branch
	// where Ruby's faraday-multipart emitted a "UTF-8 string passed as BINARY"
	// warning. Make sure no Content-Transfer-Encoding header gets attached
	// to text parts.
	dir := t.TempDir()
	picture := filepath.Join(dir, "p.png")
	if err := os.WriteFile(picture, []byte("\x89PNG\r\n\x1a\nfake"), 0o644); err != nil {
		t.Fatal(err)
	}

	var receivedBody []byte
	var receivedCT string
	c, srv := newTestClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedCT = r.Header.Get("Content-Type")
		receivedBody, _ = io.ReadAll(r.Body)
		w.WriteHeader(201)
		w.Write([]byte(`{"question":{"id":1,"name":"J1","suit":{"name":"Oops"}}}`))
	}))
	defer srv.Close()

	attrs := attrsFromFixtureActivity(env.Activities[0])
	attrs.Picture = picture
	if _, err := c.CreateQuestion(context.Background(), 42, attrs); err != nil {
		t.Fatalf("CreateQuestion: %v", err)
	}

	if !strings.HasPrefix(receivedCT, "multipart/form-data") {
		t.Fatalf("Content-Type = %q, want multipart/form-data", receivedCT)
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
		// Headers must not declare Content-Transfer-Encoding: binary on text
		// parts (the Ruby faraday gotcha called out in the handoff).
		if cte := part.Header.Get("Content-Transfer-Encoding"); cte != "" {
			t.Errorf("part %q has Content-Transfer-Encoding=%q (should be absent)", part.FormName(), cte)
		}
		b, _ := io.ReadAll(part)
		parts[part.FormName()] = string(b)
	}

	checks := []struct {
		field string
		rune  string
	}{
		{"question[text]", runeLungs},
		{"question[text]", runeMathBoldB},
		{"question[answer]", runeMathBoldn},
		{"question[assessment_notes]", runeMathBoldG},
		{"question[outcome_descriptions][]", runeTarget},
	}
	for _, ch := range checks {
		if !strings.Contains(parts[ch.field], ch.rune) {
			t.Errorf("multipart field %q missing rune %q.\ngot: %q", ch.field, ch.rune, parts[ch.field])
		}
	}

	// Lightning bolt lives in activities[1].text — verify the second activity
	// survives a separate round-trip too.
	receivedBody = nil
	attrs2 := attrsFromFixtureActivity(env.Activities[1])
	attrs2.Picture = picture
	if _, err := c.CreateQuestion(context.Background(), 42, attrs2); err != nil {
		t.Fatalf("CreateQuestion (#2): %v", err)
	}
	if !bytes.Contains(receivedBody, []byte(runeLightning)) {
		t.Errorf("multipart body for activity 2 missing lightning bolt (%x)", []byte(runeLightning))
	}
}

func TestUploadableMimeIncludesAudio(t *testing.T) {
	// Audio MIME types added to the Rails API's ALLOWED_UPLOAD_MIMES
	// whitelist as of PR #67. The CLI must hand the server the right
	// Content-Type so 415 doesn't trip.
	cases := map[string]string{
		"/tmp/x.mp3":  "audio/mpeg",
		"/tmp/x.wav":  "audio/wav",
		"/tmp/x.ogg":  "audio/ogg",
		"/tmp/x.opus": "audio/opus",
		"/tmp/x.webm": "audio/webm",
		"/tmp/x.MP3":  "audio/mpeg", // case-insensitive extension match
	}
	for path, want := range cases {
		if got := mimeFor(path); got != want {
			t.Errorf("mimeFor(%q) = %q, want %q", path, got, want)
		}
	}
}
