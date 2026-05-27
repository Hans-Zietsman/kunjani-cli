package client

import (
	"context"
	"fmt"
)

type DeckCreate struct {
	Name           string `json:"name"`
	Description    string `json:"description,omitempty"`
	Visibility     string `json:"visibility,omitempty"`
	Collaborations string `json:"collaborations,omitempty"`
	DiceOption     string `json:"dice_option,omitempty"`
	// Picture is a local file path to a deck thumbnail. Sent as multipart
	// when set; the `json:"-"` tag keeps it out of the JSON branch.
	Picture string `json:"-"`
}

// DeckUpdate carries optional deck attributes for PATCH. Every field has
// omitempty so omitted fields are not sent and the server leaves them
// untouched. Distinct from DeckCreate (where Name is required) so callers
// can't accidentally null out a deck's name with a zero-value struct.
type DeckUpdate struct {
	Name           string `json:"name,omitempty"`
	Description    string `json:"description,omitempty"`
	Visibility     string `json:"visibility,omitempty"`
	Collaborations string `json:"collaborations,omitempty"`
	DiceOption     string `json:"dice_option,omitempty"`
	// Picture is a local file path to a new deck thumbnail. Sent as
	// multipart when set. RemovePicture wipes any existing thumbnail.
	// Picture and RemovePicture are mutually exclusive — the caller is
	// responsible for not setting both at once.
	Picture       string `json:"-"`
	RemovePicture bool   `json:"-"`
}

func (c *Client) ListDecks(ctx context.Context) ([]any, error) {
	res, err := c.doJSON(ctx, "GET", "/api/v1/decks", nil)
	if err != nil {
		return nil, err
	}
	return asAnySlice(res["decks"]), nil
}

func (c *Client) CreateDeck(ctx context.Context, attrs DeckCreate) (map[string]any, error) {
	if isFile(attrs.Picture) {
		body, ct, err := buildDeckMultipart(deckCreateMultipartFields(attrs))
		if err != nil {
			return nil, err
		}
		res, err := c.doMultipart(ctx, "POST", "/api/v1/decks", ct, body)
		if err != nil {
			return nil, err
		}
		return extractDeck(res)
	}

	body := map[string]any{"deck": compactStruct(attrs)}
	res, err := c.doJSON(ctx, "POST", "/api/v1/decks", body)
	if err != nil {
		return nil, err
	}
	return extractDeck(res)
}

// GetDeck fetches a single deck by ID.
func (c *Client) GetDeck(ctx context.Context, deckID int) (map[string]any, error) {
	res, err := c.doJSON(ctx, "GET", fmt.Sprintf("/api/v1/decks/%d", deckID), nil)
	if err != nil {
		return nil, err
	}
	return extractDeck(res)
}

// UpdateDeck PATCHes the deck with the supplied attributes. Zero-value
// fields on attrs are dropped from the request thanks to omitempty; the
// server leaves them untouched.
func (c *Client) UpdateDeck(ctx context.Context, deckID int, attrs DeckUpdate) (map[string]any, error) {
	path := fmt.Sprintf("/api/v1/decks/%d", deckID)

	if isFile(attrs.Picture) || attrs.RemovePicture {
		body, ct, err := buildDeckMultipart(deckUpdateMultipartFields(attrs))
		if err != nil {
			return nil, err
		}
		res, err := c.doMultipart(ctx, "PATCH", path, ct, body)
		if err != nil {
			return nil, err
		}
		return extractDeck(res)
	}

	body := map[string]any{"deck": attrs}
	res, err := c.doJSON(ctx, "PATCH", path, body)
	if err != nil {
		return nil, err
	}
	return extractDeck(res)
}

// DeleteDeck hard-deletes the deck. Server returns 204 on success.
func (c *Client) DeleteDeck(ctx context.Context, deckID int) error {
	_, err := c.doJSON(ctx, "DELETE", fmt.Sprintf("/api/v1/decks/%d", deckID), nil)
	return err
}

// ReorderQuestions writes a new question order to the deck. The server
// validates that every ID belongs to this deck and rejects the whole batch
// otherwise. Returns the deck JSON, which echoes the saved question_order
// back so the caller can verify.
func (c *Client) ReorderQuestions(ctx context.Context, deckID int, order []int) (map[string]any, error) {
	body := map[string]any{"deck": map[string]any{"question_order": order}}
	res, err := c.doJSON(ctx, "POST", fmt.Sprintf("/api/v1/decks/%d/reorder_questions", deckID), body)
	if err != nil {
		return nil, err
	}
	return extractDeck(res)
}

func extractDeck(res map[string]any) (map[string]any, error) {
	deck, ok := res["deck"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("response missing 'deck' key")
	}
	return deck, nil
}

// deckCreateMultipartFields returns the wire fields for a deck-create
// multipart body. Empty optional fields are dropped — same shape as the
// JSON path's compactStruct.
func deckCreateMultipartFields(a DeckCreate) []deckField {
	fields := []deckField{}
	if a.Name != "" {
		fields = append(fields, deckField{name: "name", textVal: a.Name})
	}
	if a.Description != "" {
		fields = append(fields, deckField{name: "description", textVal: a.Description})
	}
	if a.Visibility != "" {
		fields = append(fields, deckField{name: "visibility", textVal: a.Visibility})
	}
	if a.Collaborations != "" {
		fields = append(fields, deckField{name: "collaborations", textVal: a.Collaborations})
	}
	if a.DiceOption != "" {
		fields = append(fields, deckField{name: "dice_option", textVal: a.DiceOption})
	}
	if isFile(a.Picture) {
		fields = append(fields, deckField{name: "picture", filePath: a.Picture})
	}
	return fields
}

// deckUpdateMultipartFields mirrors deckCreateMultipartFields but on a
// PATCH-shaped struct. Adds the remove_picture sentinel when set.
func deckUpdateMultipartFields(a DeckUpdate) []deckField {
	fields := []deckField{}
	if a.Name != "" {
		fields = append(fields, deckField{name: "name", textVal: a.Name})
	}
	if a.Description != "" {
		fields = append(fields, deckField{name: "description", textVal: a.Description})
	}
	if a.Visibility != "" {
		fields = append(fields, deckField{name: "visibility", textVal: a.Visibility})
	}
	if a.Collaborations != "" {
		fields = append(fields, deckField{name: "collaborations", textVal: a.Collaborations})
	}
	if a.DiceOption != "" {
		fields = append(fields, deckField{name: "dice_option", textVal: a.DiceOption})
	}
	if isFile(a.Picture) {
		fields = append(fields, deckField{name: "picture", filePath: a.Picture})
	}
	if a.RemovePicture {
		fields = append(fields, deckField{name: "remove_picture", textVal: "1"})
	}
	return fields
}

func compactStruct(d DeckCreate) map[string]any {
	out := map[string]any{}
	if d.Name != "" {
		out["name"] = d.Name
	}
	if d.Description != "" {
		out["description"] = d.Description
	}
	if d.Visibility != "" {
		out["visibility"] = d.Visibility
	}
	if d.Collaborations != "" {
		out["collaborations"] = d.Collaborations
	}
	if d.DiceOption != "" {
		out["dice_option"] = d.DiceOption
	}
	return out
}

func asAnySlice(v any) []any {
	if v == nil {
		return nil
	}
	if arr, ok := v.([]any); ok {
		return arr
	}
	return nil
}
