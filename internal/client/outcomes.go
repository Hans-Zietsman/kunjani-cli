package client

import (
	"context"
	"fmt"
)

type OutcomeCreate struct {
	Description    string `json:"description,omitempty"`
	QuestionIDs    []int  `json:"question_ids,omitempty"`
	HasQuestionIDs bool   `json:"-"`
}

type OutcomeUpdate struct {
	Description       string
	HasDescription    bool
	QuestionIDs       []int
	HasQuestionIDs    bool
}

func (c *Client) ListOutcomes(ctx context.Context, deckID int) ([]any, error) {
	res, err := c.doJSON(ctx, "GET", fmt.Sprintf("/api/v1/decks/%d/outcomes", deckID), nil)
	if err != nil {
		return nil, err
	}
	return asAnySlice(res["outcomes"]), nil
}

func (c *Client) CreateOutcome(ctx context.Context, deckID int, attrs OutcomeCreate) (map[string]any, error) {
	inner := map[string]any{}
	if attrs.Description != "" {
		inner["description"] = attrs.Description
	}
	if attrs.HasQuestionIDs {
		inner["question_ids"] = attrs.QuestionIDs
	}
	body := map[string]any{"outcome": inner}
	res, err := c.doJSON(ctx, "POST", fmt.Sprintf("/api/v1/decks/%d/outcomes", deckID), body)
	if err != nil {
		return nil, err
	}
	out, ok := res["outcome"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("response missing 'outcome' key")
	}
	return out, nil
}

func (c *Client) UpdateOutcome(ctx context.Context, deckID, outcomeID int, attrs OutcomeUpdate) (map[string]any, error) {
	inner := map[string]any{}
	if attrs.HasDescription {
		inner["description"] = attrs.Description
	}
	if attrs.HasQuestionIDs {
		ids := attrs.QuestionIDs
		if ids == nil {
			ids = []int{}
		}
		inner["question_ids"] = ids
	}
	body := map[string]any{"outcome": inner}
	res, err := c.doJSON(ctx, "PATCH", fmt.Sprintf("/api/v1/decks/%d/outcomes/%d", deckID, outcomeID), body)
	if err != nil {
		return nil, err
	}
	out, ok := res["outcome"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("response missing 'outcome' key")
	}
	return out, nil
}

func (c *Client) DeleteOutcome(ctx context.Context, deckID, outcomeID int) error {
	_, err := c.doJSON(ctx, "DELETE", fmt.Sprintf("/api/v1/decks/%d/outcomes/%d", deckID, outcomeID), nil)
	return err
}

func (c *Client) Me(ctx context.Context) (map[string]any, error) {
	return c.doJSON(ctx, "GET", "/api/v1/me", nil)
}
