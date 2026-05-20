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

	// Each "Has*" boolean distinguishes "flag not provided" (don't send)
	// from "flag set to empty string" (send empty to clear the field).

	ExpectedAnswerFormat    string // general | image | video | voice
	HasExpectedAnswerFormat bool

	// YouTube/Vimeo URLs. The server auto-rewrites supported URLs to
	// embed form on save. Start/end accept "MM:SS" or "S" strings.
	VideoLink          string
	HasVideoLink       bool
	VideoLinkStart     string
	HasVideoLinkStart  bool
	VideoLinkEnd       string
	HasVideoLinkEnd    bool

	VideoLink2         string
	HasVideoLink2      bool
	VideoLink2Start    string
	HasVideoLink2Start bool
	VideoLink2End      string
	HasVideoLink2End   bool

	// Response-side alternative media. Start/end are stored as integer
	// seconds server-side, but the API accepts a string and coerces.
	AnswerMedia         string
	HasAnswerMedia      bool
	AnswerMediaStart    string
	HasAnswerMediaStart bool
	AnswerMediaEnd      string
	HasAnswerMediaEnd   bool
}

// ValidAnswerFormats is the closed enum the Rails Question model accepts
// for expected_answer_format. Empty string means "leave unchanged".
var ValidAnswerFormats = []string{"general", "image", "video", "voice"}

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
	".wav":  "audio/wav",
	".ogg":  "audio/ogg",
	".opus": "audio/opus",
	".webm": "audio/webm",
	".pdf":  "application/pdf",
}

func mimeFor(path string) string {
	if m, ok := uploadableMime[strings.ToLower(filepath.Ext(path))]; ok {
		return m
	}
	return "application/octet-stream"
}

// ListQuestions returns every question on the deck, paging the API
// internally. The Rails API caps a single response at 200 (Api::V1::Base
// pagination_bounds); we ask for the server default of 50 per page so the
// pagination boundary is exercised on smaller decks too.
func (c *Client) ListQuestions(ctx context.Context, deckID int) ([]any, error) {
	const pageSize = 50
	var all []any
	offset := 0
	for {
		path := fmt.Sprintf("/api/v1/decks/%d/questions?limit=%d&offset=%d", deckID, pageSize, offset)
		res, err := c.doJSON(ctx, "GET", path, nil)
		if err != nil {
			return nil, err
		}
		page := asAnySlice(res["questions"])
		all = append(all, page...)
		if len(page) < pageSize {
			return all, nil
		}
		offset += pageSize
	}
}

// GetQuestion fetches a single question on a deck by numeric ID. The Rails
// API doesn't ship a per-question GET yet (PR open as of writing), so we
// walk the paged list. Returns *NotFoundError when the question isn't on
// the deck — same shape callers already match elsewhere.
func (c *Client) GetQuestion(ctx context.Context, deckID, questionID int) (map[string]any, error) {
	questions, err := c.ListQuestions(ctx, deckID)
	if err != nil {
		return nil, err
	}
	for _, q := range questions {
		m, ok := q.(map[string]any)
		if !ok {
			continue
		}
		// untyped JSON numbers come through as float64
		if id, ok := m["id"].(float64); ok && int(id) == questionID {
			return m, nil
		}
	}
	return nil, &NotFoundError{Msg: fmt.Sprintf("question #%d not found on deck #%d", questionID, deckID)}
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

	if a.HasExpectedAnswerFormat {
		q["expected_answer_format"] = a.ExpectedAnswerFormat
	}
	if a.HasVideoLink {
		q["video_link"] = a.VideoLink
	}
	if a.HasVideoLinkStart {
		q["video_link_start"] = a.VideoLinkStart
	}
	if a.HasVideoLinkEnd {
		q["video_link_end"] = a.VideoLinkEnd
	}
	if a.HasVideoLink2 {
		q["video_link_2"] = a.VideoLink2
	}
	if a.HasVideoLink2Start {
		q["video_link_2_start"] = a.VideoLink2Start
	}
	if a.HasVideoLink2End {
		q["video_link_2_end"] = a.VideoLink2End
	}
	if a.HasAnswerMedia {
		q["answer_media"] = a.AnswerMedia
	}
	if a.HasAnswerMediaStart {
		q["answer_media_start"] = a.AnswerMediaStart
	}
	if a.HasAnswerMediaEnd {
		q["answer_media_end"] = a.AnswerMediaEnd
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

	if a.HasExpectedAnswerFormat {
		if err := writeField("expected_answer_format", a.ExpectedAnswerFormat); err != nil {
			return nil, "", err
		}
	}
	if a.HasVideoLink {
		if err := writeField("video_link", a.VideoLink); err != nil {
			return nil, "", err
		}
	}
	if a.HasVideoLinkStart {
		if err := writeField("video_link_start", a.VideoLinkStart); err != nil {
			return nil, "", err
		}
	}
	if a.HasVideoLinkEnd {
		if err := writeField("video_link_end", a.VideoLinkEnd); err != nil {
			return nil, "", err
		}
	}
	if a.HasVideoLink2 {
		if err := writeField("video_link_2", a.VideoLink2); err != nil {
			return nil, "", err
		}
	}
	if a.HasVideoLink2Start {
		if err := writeField("video_link_2_start", a.VideoLink2Start); err != nil {
			return nil, "", err
		}
	}
	if a.HasVideoLink2End {
		if err := writeField("video_link_2_end", a.VideoLink2End); err != nil {
			return nil, "", err
		}
	}
	if a.HasAnswerMedia {
		if err := writeField("answer_media", a.AnswerMedia); err != nil {
			return nil, "", err
		}
	}
	if a.HasAnswerMediaStart {
		if err := writeField("answer_media_start", a.AnswerMediaStart); err != nil {
			return nil, "", err
		}
	}
	if a.HasAnswerMediaEnd {
		if err := writeField("answer_media_end", a.AnswerMediaEnd); err != nil {
			return nil, "", err
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
