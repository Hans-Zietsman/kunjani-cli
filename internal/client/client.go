package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

const UserAgentPrefix = "kunjani-cli/"

type Client struct {
	Host    string
	Token   string
	Version string
	HTTP    *http.Client
}

func New(host, token, version string) *Client {
	return &Client{
		Host:    strings.TrimRight(host, "/"),
		Token:   token,
		Version: version,
		HTTP:    &http.Client{Timeout: 60 * time.Second},
	}
}

func (c *Client) userAgent() string {
	v := c.Version
	if v == "" {
		v = "dev"
	}
	return UserAgentPrefix + v
}

func (c *Client) doJSON(ctx context.Context, method, path string, body any) (map[string]any, error) {
	var bodyReader io.Reader
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("marshal body: %w", err)
		}
		bodyReader = bytes.NewReader(buf)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.Host+path, bodyReader)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.Token)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", c.userAgent())
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return c.send(req, body)
}

func (c *Client) doMultipart(ctx context.Context, method, path, contentType string, body []byte) (map[string]any, error) {
	req, err := http.NewRequestWithContext(ctx, method, c.Host+path, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.Token)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", c.userAgent())
	req.Header.Set("Content-Type", contentType)
	return c.send(req, body)
}

var retryStatuses = map[int]bool{502: true, 503: true, 504: true}

func (c *Client) send(req *http.Request, replayBody any) (map[string]any, error) {
	var resp *http.Response
	var err error
	const maxAttempts = 3 // initial + 2 retries (Faraday max:2 retries)
	for attempt := 0; attempt < maxAttempts; attempt++ {
		if attempt > 0 {
			time.Sleep(500 * time.Millisecond)
			if err := rewindBody(req, replayBody); err != nil {
				return nil, err
			}
		}
		resp, err = c.HTTP.Do(req)
		if err != nil {
			if attempt == maxAttempts-1 {
				return nil, err
			}
			continue
		}
		if !retryStatuses[resp.StatusCode] {
			break
		}
		io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
		resp = nil
	}
	if resp == nil {
		return nil, err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(resp.Body)
	return handle(resp.StatusCode, raw)
}

func rewindBody(req *http.Request, replayBody any) error {
	if req.Body == nil {
		return nil
	}
	switch b := replayBody.(type) {
	case nil:
		return nil
	case []byte:
		req.Body = io.NopCloser(bytes.NewReader(b))
	default:
		buf, err := json.Marshal(replayBody)
		if err != nil {
			return err
		}
		req.Body = io.NopCloser(bytes.NewReader(buf))
	}
	return nil
}

func handle(status int, raw []byte) (map[string]any, error) {
	payload := parseBody(raw)

	switch {
	case status == 200 || status == 201:
		return payload, nil
	case status == 204:
		return payload, nil
	case status == 401:
		return nil, &AuthError{Msg: errString(payload, "Authentication failed")}
	case status == 403:
		return nil, &AuthError{Msg: errString(payload, "Forbidden")}
	case status == 404:
		return nil, &NotFoundError{Msg: errString(payload, "Not found")}
	case status == 413:
		return nil, &ValidationError{Errors: []string{errString(payload, "Upload too large")}}
	case status == 415:
		return nil, &ValidationError{Errors: []string{errString(payload, "Unsupported media type")}}
	case status == 422:
		return nil, &ValidationError{Errors: errorsArray(payload)}
	default:
		return nil, &GenericError{Msg: fmt.Sprintf("Unexpected HTTP %d: %s", status, string(raw))}
	}
}

func parseBody(raw []byte) map[string]any {
	if len(raw) == 0 {
		return map[string]any{}
	}
	var p map[string]any
	if err := json.Unmarshal(raw, &p); err != nil {
		return map[string]any{"error": string(raw)}
	}
	if p == nil {
		return map[string]any{}
	}
	return p
}

func errString(p map[string]any, fallback string) string {
	if v, ok := p["error"].(string); ok && v != "" {
		return v
	}
	return fallback
}

func errorsArray(p map[string]any) []string {
	if arr, ok := p["errors"].([]any); ok {
		out := make([]string, 0, len(arr))
		for _, v := range arr {
			out = append(out, fmt.Sprint(v))
		}
		return out
	}
	if v, ok := p["error"].(string); ok && v != "" {
		return []string{v}
	}
	return nil
}
