package client

import (
	"context"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Mirror TestCreateQuestionUsesMultipartWhenPicturePathSupplied but for
// the deck thumbnail path. Pins the wire-level shape: multipart/form-data
// with deck[name] as a plain field and deck[picture] as a file part with
// the right Content-Type.
func TestCreateDeckUsesMultipartWhenPictureSupplied(t *testing.T) {
	dir := t.TempDir()
	fixture := filepath.Join(dir, "thumb.png")
	if err := os.WriteFile(fixture, []byte("\x89PNG\r\n\x1a\nfake"), 0o644); err != nil {
		t.Fatal(err)
	}

	c, srv := newTestClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "POST" {
			t.Errorf("method = %q", r.Method)
		}
		if r.URL.Path != "/api/v1/decks" {
			t.Errorf("path = %q", r.URL.Path)
		}

		ct := r.Header.Get("Content-Type")
		mediaType, params, err := mime.ParseMediaType(ct)
		if err != nil {
			t.Fatalf("parse content-type: %v", err)
		}
		if mediaType != "multipart/form-data" {
			t.Errorf("media type = %q", mediaType)
		}

		mr := multipart.NewReader(r.Body, params["boundary"])
		gotName, gotDice, gotPicture, gotPng := false, false, false, false
		for {
			part, err := mr.NextPart()
			if err != nil {
				break
			}
			switch part.FormName() {
			case "deck[name]":
				b, _ := io.ReadAll(part)
				if string(b) == "Thumbnailed" {
					gotName = true
				}
			case "deck[dice_option]":
				b, _ := io.ReadAll(part)
				if string(b) == "loaded" {
					gotDice = true
				}
			case "deck[picture]":
				gotPicture = true
				if part.FileName() != "thumb.png" {
					t.Errorf("filename = %q", part.FileName())
				}
				if part.Header.Get("Content-Type") == "image/png" {
					gotPng = true
				}
				io.Copy(io.Discard, part)
			}
		}
		if !gotName || !gotDice || !gotPicture || !gotPng {
			t.Errorf("missing parts: name=%v dice=%v picture=%v png=%v",
				gotName, gotDice, gotPicture, gotPng)
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(201)
		w.Write([]byte(`{"deck":{"id":7,"name":"Thumbnailed","picture_url":"https://example/uploads/x.png"}}`))
	}))
	defer srv.Close()

	d, err := c.CreateDeck(context.Background(), DeckCreate{
		Name:       "Thumbnailed",
		DiceOption: "loaded",
		Picture:    fixture,
	})
	if err != nil {
		t.Fatal(err)
	}
	if d["id"].(float64) != 7 {
		t.Errorf("id = %v", d["id"])
	}
	if d["picture_url"] != "https://example/uploads/x.png" {
		t.Errorf("picture_url = %v", d["picture_url"])
	}
}

// When no --picture is passed, the JSON branch fires — no multipart
// boundary, no file open attempt. Locks the dual-mode dispatch.
func TestCreateDeckUsesJSONWhenNoPicture(t *testing.T) {
	c, srv := newTestClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ct := r.Header.Get("Content-Type")
		if !strings.HasPrefix(ct, "application/json") {
			t.Errorf("Content-Type = %q, want application/json", ct)
		}
		body := readJSON(t, r)
		deck := body["deck"].(map[string]any)
		if deck["name"] != "PlainDeck" {
			t.Errorf("name = %v", deck["name"])
		}
		if _, hasPicture := deck["picture"]; hasPicture {
			t.Errorf("deck.picture should not appear in JSON body")
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(201)
		w.Write([]byte(`{"deck":{"id":1,"name":"PlainDeck"}}`))
	}))
	defer srv.Close()

	if _, err := c.CreateDeck(context.Background(), DeckCreate{Name: "PlainDeck"}); err != nil {
		t.Fatal(err)
	}
}

func TestUpdateDeckUsesMultipartWhenPictureSupplied(t *testing.T) {
	dir := t.TempDir()
	fixture := filepath.Join(dir, "new-thumb.png")
	if err := os.WriteFile(fixture, []byte("\x89PNG\r\n\x1a\nfake"), 0o644); err != nil {
		t.Fatal(err)
	}

	c, srv := newTestClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != "PATCH" {
			t.Errorf("method = %q", r.Method)
		}
		if r.URL.Path != "/api/v1/decks/42" {
			t.Errorf("path = %q", r.URL.Path)
		}

		ct := r.Header.Get("Content-Type")
		mediaType, params, err := mime.ParseMediaType(ct)
		if err != nil {
			t.Fatalf("parse content-type: %v", err)
		}
		if mediaType != "multipart/form-data" {
			t.Errorf("media type = %q", mediaType)
		}

		mr := multipart.NewReader(r.Body, params["boundary"])
		gotPicture := false
		for {
			part, err := mr.NextPart()
			if err != nil {
				break
			}
			if part.FormName() == "deck[picture]" {
				gotPicture = true
				if part.FileName() != "new-thumb.png" {
					t.Errorf("filename = %q", part.FileName())
				}
				io.Copy(io.Discard, part)
			}
		}
		if !gotPicture {
			t.Errorf("expected deck[picture] part in PATCH body")
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		w.Write([]byte(`{"deck":{"id":42,"name":"X","picture_url":"https://example/y.png"}}`))
	}))
	defer srv.Close()

	if _, err := c.UpdateDeck(context.Background(), 42, DeckUpdate{Picture: fixture}); err != nil {
		t.Fatal(err)
	}
}

// remove-picture flips to multipart so the form carries the sentinel.
// Multipart is the right channel because Dragonfly's controller-side
// reader looks at the same Rails params hash regardless — but keeping
// the wire shape consistent (multipart whenever a file-side field
// fires) makes the dispatch logic one branch, not two.
func TestUpdateDeckRemovePictureSendsSentinelOverMultipart(t *testing.T) {
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
		gotRemove := false
		for {
			part, err := mr.NextPart()
			if err != nil {
				break
			}
			if part.FormName() == "deck[remove_picture]" {
				b, _ := io.ReadAll(part)
				if string(b) == "1" {
					gotRemove = true
				}
			}
		}
		if !gotRemove {
			t.Errorf("expected deck[remove_picture]=1 sentinel")
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		w.Write([]byte(`{"deck":{"id":42,"name":"X","picture_url":null}}`))
	}))
	defer srv.Close()

	if _, err := c.UpdateDeck(context.Background(), 42, DeckUpdate{RemovePicture: true}); err != nil {
		t.Fatal(err)
	}
}

// When the user doesn't pass --picture or --remove-picture, update-deck
// must keep using the JSON branch. Guards against an accidental
// always-multipart regression.
func TestUpdateDeckUsesJSONWhenNoPictureChanges(t *testing.T) {
	c, srv := newTestClient(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ct := r.Header.Get("Content-Type")
		if !strings.HasPrefix(ct, "application/json") {
			t.Errorf("Content-Type = %q, want application/json", ct)
		}
		body := readJSON(t, r)
		deck := body["deck"].(map[string]any)
		if deck["dice_option"] != "random" {
			t.Errorf("dice_option = %v", deck["dice_option"])
		}

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(200)
		w.Write([]byte(`{"deck":{"id":42,"name":"X","dice_option":"random"}}`))
	}))
	defer srv.Close()

	if _, err := c.UpdateDeck(context.Background(), 42, DeckUpdate{DiceOption: "random"}); err != nil {
		t.Fatal(err)
	}
}
