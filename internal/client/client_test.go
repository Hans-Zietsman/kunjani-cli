package client

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func newTestClient(handler http.Handler) (*Client, *httptest.Server) {
	srv := httptest.NewServer(handler)
	c := New(srv.URL, "knj_test", "test")
	return c, srv
}

func readJSON(t *testing.T, r *http.Request) map[string]any {
	t.Helper()
	body, err := io.ReadAll(r.Body)
	if err != nil {
		t.Fatalf("read body: %v", err)
	}
	var m map[string]any
	if len(body) > 0 {
		if err := json.Unmarshal(body, &m); err != nil {
			t.Fatalf("parse body: %v body=%s", err, body)
		}
	}
	return m
}

func TestMeReturnsParsedUser(t *testing.T) {
	c, srv := newTestClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/me" {
			t.Errorf("path = %q", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer knj_test" {
			t.Errorf("Authorization = %q", got)
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		w.Write([]byte(`{"id":1,"email":"a@b.c","role":"facilitator"}`))
	}))
	defer srv.Close()

	me, err := c.Me(context.Background())
	if err != nil {
		t.Fatalf("Me: %v", err)
	}
	if me["email"] != "a@b.c" {
		t.Errorf("email = %v", me["email"])
	}
}

func TestUnauthorizedRaisesAuthError(t *testing.T) {
	c, srv := newTestClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(401)
		w.Write([]byte(`{"error":"bad token"}`))
	}))
	defer srv.Close()

	_, err := c.Me(context.Background())
	var authErr *AuthError
	if !errors.As(err, &authErr) {
		t.Fatalf("err type = %T (%v)", err, err)
	}
	if !strings.Contains(authErr.Msg, "bad token") {
		t.Errorf("msg = %q", authErr.Msg)
	}
}

func TestListDecksReturnsArray(t *testing.T) {
	c, srv := newTestClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"decks":[{"id":1,"name":"A"}]}`))
	}))
	defer srv.Close()

	decks, err := c.ListDecks(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(decks) != 1 {
		t.Fatalf("len = %d", len(decks))
	}
	d := decks[0].(map[string]any)
	if d["name"] != "A" {
		t.Errorf("name = %v", d["name"])
	}
}

func TestCreateDeckStripsNilValuesAndReturnsDeck(t *testing.T) {
	c, srv := newTestClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body := readJSON(t, r)
		deck := body["deck"].(map[string]any)
		if _, has := deck["description"]; has {
			t.Errorf("description should be omitted")
		}
		if deck["name"] != "Pilot" {
			t.Errorf("name = %v", deck["name"])
		}
		w.WriteHeader(201)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"deck":{"id":42,"name":"Pilot"}}`))
	}))
	defer srv.Close()

	deck, err := c.CreateDeck(context.Background(), DeckCreate{Name: "Pilot"})
	if err != nil {
		t.Fatal(err)
	}
	if deck["id"].(float64) != 42 {
		t.Errorf("id = %v", deck["id"])
	}
}

func TestCreateQuestionPostsToNestedRoute(t *testing.T) {
	c, srv := newTestClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/decks/42/questions" {
			t.Errorf("path = %q", r.URL.Path)
		}
		body := readJSON(t, r)
		q := body["question"].(map[string]any)
		if q["suit_name"] != "Jolt" || q["text"] != "Why?" || q["answer"] != "Because" {
			t.Errorf("body = %v", q)
		}
		w.WriteHeader(201)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"question":{"id":7,"name":"J1","suit":{"name":"Jolt"}}}`))
	}))
	defer srv.Close()

	q, err := c.CreateQuestion(context.Background(), 42, QuestionAttrs{
		SuitName: "Jolt", Text: "Why?", Answer: "Because",
	})
	if err != nil {
		t.Fatal(err)
	}
	if q["id"].(float64) != 7 {
		t.Errorf("id = %v", q["id"])
	}
}

func TestValidationErrorRaisesWithErrorsArray(t *testing.T) {
	c, srv := newTestClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(422)
		w.Write([]byte(`{"errors":["Name is missing","Other thing"]}`))
	}))
	defer srv.Close()

	_, err := c.CreateDeck(context.Background(), DeckCreate{Name: ""})
	var vErr *ValidationError
	if !errors.As(err, &vErr) {
		t.Fatalf("err type = %T (%v)", err, err)
	}
	found := false
	for _, e := range vErr.Errors {
		if e == "Name is missing" {
			found = true
		}
	}
	if !found {
		t.Errorf("errors = %v", vErr.Errors)
	}
}

func TestNotFoundRaises(t *testing.T) {
	c, srv := newTestClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
		w.Write([]byte(`{"error":"no deck"}`))
	}))
	defer srv.Close()

	_, err := c.ListQuestions(context.Background(), 9999)
	var nf *NotFoundError
	if !errors.As(err, &nf) {
		t.Fatalf("err type = %T (%v)", err, err)
	}
}

func TestCreateQuestionUsesMultipartWhenPicturePathSupplied(t *testing.T) {
	dir := t.TempDir()
	fixture := filepath.Join(dir, "tiny.png")
	if err := os.WriteFile(fixture, []byte("\x89PNG\r\n\x1a\nfake"), 0o644); err != nil {
		t.Fatal(err)
	}

	c, srv := newTestClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ct := r.Header.Get("Content-Type")
		mediaType, params, err := mime.ParseMediaType(ct)
		if err != nil {
			t.Fatalf("parse content-type: %v", err)
		}
		if mediaType != "multipart/form-data" {
			t.Errorf("media type = %q", mediaType)
		}
		mr := multipart.NewReader(r.Body, params["boundary"])
		gotSuit := false
		gotPicture := false
		gotPng := false
		for {
			part, err := mr.NextPart()
			if err != nil {
				break
			}
			name := part.FormName()
			if name == "question[suit_name]" {
				b, _ := io.ReadAll(part)
				if string(b) == "Jolt" {
					gotSuit = true
				}
			}
			if name == "question[picture]" {
				gotPicture = true
				if part.FileName() != "tiny.png" {
					t.Errorf("filename = %q", part.FileName())
				}
				if part.Header.Get("Content-Type") == "image/png" {
					gotPng = true
				}
				io.Copy(io.Discard, part)
			}
		}
		if !gotSuit || !gotPicture || !gotPng {
			t.Errorf("missing parts: suit=%v picture=%v png=%v", gotSuit, gotPicture, gotPng)
		}
		w.WriteHeader(201)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"question":{"id":99,"name":"J1","suit":{"name":"Jolt"}}}`))
	}))
	defer srv.Close()

	q, err := c.CreateQuestion(context.Background(), 42, QuestionAttrs{
		SuitName: "Jolt", Text: "X", Answer: "Y", Picture: fixture,
	})
	if err != nil {
		t.Fatal(err)
	}
	if q["id"].(float64) != 99 {
		t.Errorf("id = %v", q["id"])
	}
}

func TestUpdateQuestionSendsPatchToNestedRoute(t *testing.T) {
	c, srv := newTestClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "PATCH" {
			t.Errorf("method = %q", r.Method)
		}
		if r.URL.Path != "/api/v1/decks/42/questions/99" {
			t.Errorf("path = %q", r.URL.Path)
		}
		body := readJSON(t, r)
		q := body["question"].(map[string]any)
		if q["text"] != "fixed typo" {
			t.Errorf("text = %v", q["text"])
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"question":{"id":99,"name":"J1","text":"fixed typo","suit":{"name":"Jolt"}}}`))
	}))
	defer srv.Close()

	q, err := c.UpdateQuestion(context.Background(), 42, 99, QuestionAttrs{Text: "fixed typo"})
	if err != nil {
		t.Fatal(err)
	}
	if q["text"] != "fixed typo" {
		t.Errorf("text = %v", q["text"])
	}
}

func TestPayloadTooLargeRaisesValidationError(t *testing.T) {
	c, srv := newTestClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(413)
		w.Write([]byte(`{"error":"picture is 16MB (max 15MB)"}`))
	}))
	defer srv.Close()

	_, err := c.CreateQuestion(context.Background(), 42, QuestionAttrs{SuitName: "Jolt", Text: "x", Answer: "y"})
	var vErr *ValidationError
	if !errors.As(err, &vErr) {
		t.Fatalf("err type = %T (%v)", err, err)
	}
	if len(vErr.Errors) == 0 || !strings.Contains(vErr.Errors[0], "16MB") {
		t.Errorf("errors = %v", vErr.Errors)
	}
}

func TestUnsupportedMediaTypeRaisesValidationError(t *testing.T) {
	c, srv := newTestClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(415)
		w.Write([]byte(`{"error":"picture has unsupported content type 'text/plain'"}`))
	}))
	defer srv.Close()

	_, err := c.CreateQuestion(context.Background(), 42, QuestionAttrs{SuitName: "Jolt", Text: "x", Answer: "y"})
	var vErr *ValidationError
	if !errors.As(err, &vErr) {
		t.Fatalf("err type = %T (%v)", err, err)
	}
	if !strings.Contains(vErr.Errors[0], "text/plain") {
		t.Errorf("errors = %v", vErr.Errors)
	}
}

func TestListOutcomesReturnsArray(t *testing.T) {
	c, srv := newTestClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"deck_id":42,"outcomes":[{"id":1,"description":"A","kind":"direct","question_ids":[]}]}`))
	}))
	defer srv.Close()

	outcomes, err := c.ListOutcomes(context.Background(), 42)
	if err != nil {
		t.Fatal(err)
	}
	if len(outcomes) != 1 {
		t.Fatalf("len = %d", len(outcomes))
	}
	o := outcomes[0].(map[string]any)
	if o["description"] != "A" || o["kind"] != "direct" {
		t.Errorf("outcome = %v", o)
	}
}

func TestCreateOutcomePostsToNestedRoute(t *testing.T) {
	c, srv := newTestClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body := readJSON(t, r)
		o := body["outcome"].(map[string]any)
		if o["description"] != "Cat Diets" {
			t.Errorf("description = %v", o["description"])
		}
		ids := o["question_ids"].([]any)
		if len(ids) != 2 || int(ids[0].(float64)) != 1 || int(ids[1].(float64)) != 2 {
			t.Errorf("ids = %v", ids)
		}
		w.WriteHeader(201)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"outcome":{"id":99,"description":"Cat Diets","question_ids":[1,2]}}`))
	}))
	defer srv.Close()

	out, err := c.CreateOutcome(context.Background(), 42, OutcomeCreate{
		Description: "Cat Diets", QuestionIDs: []int{1, 2}, HasQuestionIDs: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if out["id"].(float64) != 99 {
		t.Errorf("id = %v", out["id"])
	}
}

func TestCreateOutcomeStripsNilQuestionIDs(t *testing.T) {
	c, srv := newTestClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body := readJSON(t, r)
		o := body["outcome"].(map[string]any)
		if _, has := o["question_ids"]; has {
			t.Errorf("question_ids should be absent")
		}
		w.WriteHeader(201)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"outcome":{"id":100,"description":"Just description","question_ids":[]}}`))
	}))
	defer srv.Close()

	out, err := c.CreateOutcome(context.Background(), 42, OutcomeCreate{Description: "Just description"})
	if err != nil {
		t.Fatal(err)
	}
	if out["id"].(float64) != 100 {
		t.Errorf("id = %v", out["id"])
	}
}

func TestUpdateOutcomePatchesNestedRoute(t *testing.T) {
	c, srv := newTestClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body := readJSON(t, r)
		o := body["outcome"].(map[string]any)
		if o["description"] != "Renamed" {
			t.Errorf("description = %v", o["description"])
		}
		ids := o["question_ids"].([]any)
		if len(ids) != 1 || int(ids[0].(float64)) != 3 {
			t.Errorf("ids = %v", ids)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"outcome":{"id":99,"description":"Renamed","question_ids":[3]}}`))
	}))
	defer srv.Close()

	out, err := c.UpdateOutcome(context.Background(), 42, 99, OutcomeUpdate{
		Description: "Renamed", HasDescription: true,
		QuestionIDs: []int{3}, HasQuestionIDs: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if out["description"] != "Renamed" {
		t.Errorf("description = %v", out["description"])
	}
}

func TestDeleteOutcomeReturnsNilOn204(t *testing.T) {
	c, srv := newTestClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "DELETE" {
			t.Errorf("method = %q", r.Method)
		}
		w.WriteHeader(204)
	}))
	defer srv.Close()

	if err := c.DeleteOutcome(context.Background(), 42, 99); err != nil {
		t.Errorf("DeleteOutcome: %v", err)
	}
}

func TestDeleteOutcome404Raises(t *testing.T) {
	c, srv := newTestClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(404)
		w.Write([]byte(`{"error":"gone"}`))
	}))
	defer srv.Close()

	err := c.DeleteOutcome(context.Background(), 42, 999)
	var nf *NotFoundError
	if !errors.As(err, &nf) {
		t.Fatalf("err type = %T (%v)", err, err)
	}
}

func TestCreateQuestionWithOutcomeDescriptionsSendsArrayInJSON(t *testing.T) {
	c, srv := newTestClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body := readJSON(t, r)
		q := body["question"].(map[string]any)
		descs := q["outcome_descriptions"].([]any)
		if len(descs) != 2 || descs[0] != "Cat Diets" || descs[1] != "Cat Habits" {
			t.Errorf("descs = %v", descs)
		}
		w.WriteHeader(201)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"question":{"id":7,"name":"J1","suit":{"name":"Jolt"},"outcomes":[]}}`))
	}))
	defer srv.Close()

	q, err := c.CreateQuestion(context.Background(), 42, QuestionAttrs{
		SuitName: "Jolt", Text: "x", Answer: "y",
		OutcomeDescriptions:    []string{"Cat Diets", "Cat Habits"},
		HasOutcomeDescriptions: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if q["id"].(float64) != 7 {
		t.Errorf("id = %v", q["id"])
	}
}

func TestCreateQuestionMultipartSendsOutcomeDescriptionsAsRepeatedFields(t *testing.T) {
	dir := t.TempDir()
	fixture := filepath.Join(dir, "tiny.png")
	if err := os.WriteFile(fixture, []byte("\x89PNG\r\n\x1a\nfake"), 0o644); err != nil {
		t.Fatal(err)
	}

	c, srv := newTestClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ct := r.Header.Get("Content-Type")
		_, params, err := mime.ParseMediaType(ct)
		if err != nil {
			t.Fatal(err)
		}
		mr := multipart.NewReader(r.Body, params["boundary"])
		descs := []string{}
		hasPicture := false
		for {
			part, err := mr.NextPart()
			if err != nil {
				break
			}
			if part.FormName() == "question[outcome_descriptions][]" {
				b, _ := io.ReadAll(part)
				descs = append(descs, string(b))
			}
			if part.FormName() == "question[picture]" {
				hasPicture = true
				io.Copy(io.Discard, part)
			}
		}
		if len(descs) != 2 || descs[0] != "Cat Diets" || descs[1] != "Cat Habits" {
			t.Errorf("descs = %v", descs)
		}
		if !hasPicture {
			t.Error("missing picture part")
		}
		w.WriteHeader(201)
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"question":{"id":99,"name":"J1","suit":{"name":"Jolt"}}}`))
	}))
	defer srv.Close()

	q, err := c.CreateQuestion(context.Background(), 42, QuestionAttrs{
		SuitName: "Jolt", Text: "x", Answer: "y", Picture: fixture,
		OutcomeDescriptions:    []string{"Cat Diets", "Cat Habits"},
		HasOutcomeDescriptions: true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if q["id"].(float64) != 99 {
		t.Errorf("id = %v", q["id"])
	}
}

func TestUpdateQuestionWithEmptyOutcomeDescriptionsClearsThem(t *testing.T) {
	c, srv := newTestClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body := readJSON(t, r)
		q := body["question"].(map[string]any)
		descs, ok := q["outcome_descriptions"].([]any)
		if !ok {
			t.Fatalf("outcome_descriptions missing or wrong type: %v", q["outcome_descriptions"])
		}
		if len(descs) != 0 {
			t.Errorf("expected empty array, got %v", descs)
		}
		w.Header().Set("Content-Type", "application/json")
		w.Write([]byte(`{"question":{"id":99,"name":"J1","suit":{"name":"Jolt"},"outcomes":[]}}`))
	}))
	defer srv.Close()

	q, err := c.UpdateQuestion(context.Background(), 42, 99, QuestionAttrs{
		HasOutcomeDescriptions: true,
		OutcomeDescriptions:    []string{},
	})
	if err != nil {
		t.Fatal(err)
	}
	if q["id"].(float64) != 99 {
		t.Errorf("id = %v", q["id"])
	}
}
