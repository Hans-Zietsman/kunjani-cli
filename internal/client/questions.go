package client

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"mime/multipart"
	"net/textproto"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type QuestionAttrs struct {
	SuitName                 string
	Text                     string
	Answer                   string
	Name                     string
	TimeInSeconds            *int
	AssessmentNotes          string
	Picture                  string
	FacilitatorPicture       string
	OutcomeDescriptions      []string
	HasOutcomeDescriptions   bool
	RemovePicture            bool
	RemoveFacilitatorPicture bool
}

var uploadableMime = map[string]string{
	".jpg":  "image/jpeg",
	".jpeg": "image/jpeg",
	".png":  "image/png",
	".gif":  "image/gif",
	".webp": "image/webp",
	".svg":  "image/svg+xml",
	".bmp":  "image/bmp",
	".tiff": "image/tiff",
	".tif":  "image/tiff",
	".mp3":  "audio/mpeg",
	".mp4":  "video/mp4",
	".pdf":  "application/pdf",
}

func mimeFor(path string) string {
	if m, ok := uploadableMime[strings.ToLower(filepath.Ext(path))]; ok {
		return m
	}
	return "application/octet-stream"
}

func (c *Client) ListQuestions(ctx context.Context, deckID int) ([]any, error) {
	res, err := c.doJSON(ctx, "GET", fmt.Sprintf("/api/v1/decks/%d/questions", deckID), nil)
	if err != nil {
		return nil, err
	}
	return asAnySlice(res["questions"]), nil
}

func (c *Client) CreateQuestion(ctx context.Context, deckID int, attrs QuestionAttrs) (map[string]any, error) {
	path := fmt.Sprintf("/api/v1/decks/%d/questions", deckID)
	return c.sendQuestion(ctx, "POST", path, attrs)
}

func (c *Client) UpdateQuestion(ctx context.Context, deckID, questionID int, attrs QuestionAttrs) (map[string]any, error) {
	path := fmt.Sprintf("/api/v1/decks/%d/questions/%d", deckID, questionID)
	return c.sendQuestion(ctx, "PATCH", path, attrs)
}

func (c *Client) sendQuestion(ctx context.Context, method, path string, attrs QuestionAttrs) (map[string]any, error) {
	hasPicture := isFile(attrs.Picture)
	hasFacilitator := isFile(attrs.FacilitatorPicture)

	if hasPicture || hasFacilitator {
		body, ct, err := buildMultipart(attrs)
		if err != nil {
			return nil, err
		}
		res, err := c.doMultipart(ctx, method, path, ct, body)
		if err != nil {
			return nil, err
		}
		q, ok := res["question"].(map[string]any)
		if !ok {
			return nil, fmt.Errorf("response missing 'question' key")
		}
		return q, nil
	}

	body := map[string]any{"question": questionJSONBody(attrs)}
	res, err := c.doJSON(ctx, method, path, body)
	if err != nil {
		return nil, err
	}
	q, ok := res["question"].(map[string]any)
	if !ok {
		return nil, fmt.Errorf("response missing 'question' key")
	}
	return q, nil
}

func isFile(path string) bool {
	if path == "" {
		return false
	}
	info, err := os.Stat(path)
	if err != nil {
		return false
	}
	return !info.IsDir()
}

func questionJSONBody(a QuestionAttrs) map[string]any {
	q := map[string]any{}
	if a.SuitName != "" {
		q["suit_name"] = a.SuitName
	}
	if a.Text != "" {
		q["text"] = a.Text
	}
	if a.Answer != "" {
		q["answer"] = a.Answer
	}
	if a.Name != "" {
		q["name"] = a.Name
	}
	if a.TimeInSeconds != nil {
		q["time_in_seconds"] = *a.TimeInSeconds
	}
	if a.AssessmentNotes != "" {
		q["assessment_notes"] = a.AssessmentNotes
	}
	if a.RemovePicture {
		q["remove_picture"] = "1"
	}
	if a.RemoveFacilitatorPicture {
		q["remove_facilitator_picture"] = "1"
	}
	if a.HasOutcomeDescriptions {
		descs := a.OutcomeDescriptions
		if descs == nil {
			descs = []string{}
		}
		q["outcome_descriptions"] = descs
	}
	return q
}

func buildMultipart(a QuestionAttrs) ([]byte, string, error) {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)

	writeField := func(key, value string) error {
		return w.WriteField("question["+key+"]", value)
	}

	if a.SuitName != "" {
		if err := writeField("suit_name", a.SuitName); err != nil {
			return nil, "", err
		}
	}
	if a.Text != "" {
		if err := writeField("text", a.Text); err != nil {
			return nil, "", err
		}
	}
	if a.Answer != "" {
		if err := writeField("answer", a.Answer); err != nil {
			return nil, "", err
		}
	}
	if a.Name != "" {
		if err := writeField("name", a.Name); err != nil {
			return nil, "", err
		}
	}
	if a.TimeInSeconds != nil {
		if err := writeField("time_in_seconds", strconv.Itoa(*a.TimeInSeconds)); err != nil {
			return nil, "", err
		}
	}
	if a.AssessmentNotes != "" {
		if err := writeField("assessment_notes", a.AssessmentNotes); err != nil {
			return nil, "", err
		}
	}
	if a.RemovePicture {
		if err := writeField("remove_picture", "1"); err != nil {
			return nil, "", err
		}
	}
	if a.RemoveFacilitatorPicture {
		if err := writeField("remove_facilitator_picture", "1"); err != nil {
			return nil, "", err
		}
	}

	if a.HasOutcomeDescriptions {
		for _, d := range a.OutcomeDescriptions {
			if err := w.WriteField("question[outcome_descriptions][]", d); err != nil {
				return nil, "", err
			}
		}
	}

	if isFile(a.Picture) {
		if err := writeFilePart(w, "question[picture]", a.Picture); err != nil {
			return nil, "", err
		}
	}
	if isFile(a.FacilitatorPicture) {
		if err := writeFilePart(w, "question[facilitator_picture]", a.FacilitatorPicture); err != nil {
			return nil, "", err
		}
	}

	if err := w.Close(); err != nil {
		return nil, "", err
	}
	return buf.Bytes(), w.FormDataContentType(), nil
}

func writeFilePart(w *multipart.Writer, fieldName, path string) error {
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	h := textproto.MIMEHeader{}
	h.Set("Content-Disposition", fmt.Sprintf(`form-data; name=%q; filename=%q`, fieldName, filepath.Base(path)))
	h.Set("Content-Type", mimeFor(path))

	part, err := w.CreatePart(h)
	if err != nil {
		return err
	}
	_, err = io.Copy(part, f)
	return err
}
