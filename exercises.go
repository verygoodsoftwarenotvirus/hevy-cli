package hevy

import (
	"context"
	"fmt"
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

// CreateExerciseTemplate creates a custom exercise template.
func (c *Client) CreateExerciseTemplate(ctx context.Context, req *ExerciseTemplateRequest) (string, error) {
	body := struct {
		Exercise *ExerciseTemplateRequest `json:"exercise"`
	}{Exercise: req}

	var resp createExerciseTemplateResponse
	if err := c.doJSON(ctx, http.MethodPost, "/v1/exercise_templates", body, &resp); err != nil {
		return "", fmt.Errorf("creating exercise template: %w", err)
	}
	return resp.ID, nil
}
