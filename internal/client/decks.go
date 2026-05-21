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
