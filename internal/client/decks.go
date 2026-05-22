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
}

func (c *Client) ListDecks(ctx context.Context) ([]any, error) {
	res, err := c.doJSON(ctx, "GET", "/api/v1/decks", nil)
	if err != nil {
		return nil, err
	}
	return asAnySlice(res["decks"]), nil
}

func (c *Client) CreateDeck(ctx context.Context, attrs DeckCreate) (map[string]any, error) {
	body := map[string]any{"deck": compactStruct(attrs)}
	res, err := c.doJSON(ctx, "POST", "/api/v1/decks", body)
	if err != nil {
		return nil, err
	}
	deck, ok := res["deck"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("response missing 'deck' key")
	}
	return deck, nil
}

// GetDeck fetches a single deck by ID.
func (c *Client) GetDeck(ctx context.Context, deckID int) (map[string]any, error) {
	res, err := c.doJSON(ctx, "GET", fmt.Sprintf("/api/v1/decks/%d", deckID), nil)
	if err != nil {
		return nil, err
	}
	deck, ok := res["deck"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("response missing 'deck' key")
	}
	return deck, nil
}

// UpdateDeck PATCHes the deck with the supplied attributes. Zero-value
// fields on attrs are dropped from the request thanks to omitempty; the
// server leaves them untouched.
func (c *Client) UpdateDeck(ctx context.Context, deckID int, attrs DeckUpdate) (map[string]any, error) {
	body := map[string]any{"deck": attrs}
	res, err := c.doJSON(ctx, "PATCH", fmt.Sprintf("/api/v1/decks/%d", deckID), body)
	if err != nil {
		return nil, err
	}
	deck, ok := res["deck"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("response missing 'deck' key")
	}
	return deck, nil
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
	deck, ok := res["deck"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("response missing 'deck' key")
	}
	return deck, nil
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
