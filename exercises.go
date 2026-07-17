package hevy

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"iter"
	"net/http"
	"net/url"
	"strconv"
)

// GetExerciseTemplate retrieves a single exercise template by ID.
func (c *Client) GetExerciseTemplate(ctx context.Context, id string) (*ExerciseTemplate, error) {
	var t ExerciseTemplate
	if err := c.doJSON(ctx, http.MethodGet, "/v1/exercise_templates/"+id, nil, &t); err != nil {
		return nil, err
	}
	return &t, nil
}

// ListExerciseTemplates returns an iterator over all exercise templates, handling pagination automatically.
func (c *Client) ListExerciseTemplates(ctx context.Context) iter.Seq2[ExerciseTemplate, error] {
	return fetchAllPages(ctx, func(ctx context.Context, page int) ([]ExerciseTemplate, int, error) {
		params := url.Values{}
		params.Set("page", strconv.Itoa(page))
		params.Set("pageSize", "100")

		var resp exerciseTemplatesResponse
		if err := c.doJSON(ctx, http.MethodGet, "/v1/exercise_templates?"+params.Encode(), nil, &resp); err != nil {
			return nil, 0, err
		}
		return resp.ExerciseTemplates, resp.PageCount, nil
	})
}

// CreateExerciseTemplate creates a custom exercise template and returns its ID.
func (c *Client) CreateExerciseTemplate(ctx context.Context, req *ExerciseTemplateRequest) (string, error) {
	body := struct {
		Exercise *ExerciseTemplateRequest `json:"exercise"`
	}{Exercise: req}

	data, err := json.Marshal(body)
	if err != nil {
		return "", fmt.Errorf("marshaling request: %w", err)
	}

	httpReq, err := c.newRequest(ctx, http.MethodPost, "/v1/exercise_templates", bytes.NewReader(data))
	if err != nil {
		return "", err
	}

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("executing request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", newAPIError(resp)
	}

	respBody, err := io.ReadAll(io.LimitReader(resp.Body, 4096))
	if err != nil {
		return "", fmt.Errorf("reading response: %w", err)
	}

	id, err := parseCreatedTemplateID(respBody)
	if err != nil {
		return "", fmt.Errorf("creating exercise template: %w", err)
	}
	return id, nil
}

// parseCreatedTemplateID extracts the new template's ID from the create response. The
// Hevy API returns the ID as a bare string (e.g. `fd014e9e-...`); this also tolerates a
// JSON-quoted string or a wrapped {"id": "..."} object defensively.
func parseCreatedTemplateID(body []byte) (string, error) {
	trimmed := bytes.TrimSpace(body)
	if len(trimmed) == 0 {
		return "", fmt.Errorf("empty response body")
	}

	if trimmed[0] == '{' {
		var resp createExerciseTemplateResponse
		if err := json.Unmarshal(trimmed, &resp); err != nil {
			return "", fmt.Errorf("decoding response %q: %w", trimmed, err)
		}
		if resp.ID == "" {
			return "", fmt.Errorf("response %q has no id", trimmed)
		}
		return resp.ID, nil
	}

	// Bare (optionally quoted) id string.
	id := string(bytes.Trim(trimmed, `"`))
	if id == "" {
		return "", fmt.Errorf("response %q has no id", trimmed)
	}
	return id, nil
}
