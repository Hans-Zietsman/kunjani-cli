package client

import (
	"bytes"
	"mime/multipart"
)

// deckField captures one wire-level field for a deck multipart body.
// textVal is empty when filePath is set, and vice versa — never both.
type deckField struct {
	name     string
	textVal  string
	filePath string
}

// buildDeckMultipart writes the supplied fields into a `deck[...]` multipart
// body and returns the bytes + Content-Type. File parts go through the
// shared writeFilePart helper so the deck and question upload paths emit
// the same Content-Disposition / filename shape.
func buildDeckMultipart(fields []deckField) ([]byte, string, error) {
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)

	for _, f := range fields {
		if f.filePath != "" {
			if err := writeFilePart(w, "deck["+f.name+"]", f.filePath); err != nil {
				return nil, "", err
			}
			continue
		}
		if err := w.WriteField("deck["+f.name+"]", f.textVal); err != nil {
			return nil, "", err
		}
	}

	if err := w.Close(); err != nil {
		return nil, "", err
	}
	return buf.Bytes(), w.FormDataContentType(), nil
}
