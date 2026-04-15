package hevy

import (
	"context"
	"fmt"
	"iter"
	"net/http"
	"net/url"
	"strconv"
)

// GetRoutine retrieves a single routine by ID.
func (c *Client) GetRoutine(ctx context.Context, id string) (*Routine, error) {
	var resp singleRoutineResponse
	if err := c.doJSON(ctx, http.MethodGet, "/v1/routines/"+id, nil, &resp); err != nil {
		return nil, err
	}
	return &resp.Routine, nil
}

// ListRoutines returns an iterator over all routines, handling pagination automatically.
func (c *Client) ListRoutines(ctx context.Context) iter.Seq2[Routine, error] {
	return fetchAllPages(ctx, func(ctx context.Context, page int) ([]Routine, int, error) {
		params := url.Values{}
		params.Set("page", strconv.Itoa(page))
		params.Set("pageSize", "10")

		var resp routinesResponse
		if err := c.doJSON(ctx, http.MethodGet, "/v1/routines?"+params.Encode(), nil, &resp); err != nil {
			return nil, 0, err
		}
		return resp.Routines, resp.PageCount, nil
	})
}

// CreateRoutine creates a new routine.
func (c *Client) CreateRoutine(ctx context.Context, req *RoutineRequest) (*Routine, error) {
	body := struct {
		Routine *RoutineRequest `json:"routine"`
	}{Routine: req}

	var resp struct {
		Routine []Routine `json:"routine"`
	}
	if err := c.doJSON(ctx, http.MethodPost, "/v1/routines", body, &resp); err != nil {
		return nil, fmt.Errorf("creating routine: %w", err)
	}
	if len(resp.Routine) == 0 {
		return nil, fmt.Errorf("creating routine: empty response")
	}
	return &resp.Routine[0], nil
}

// UpdateRoutine updates an existing routine.
func (c *Client) UpdateRoutine(ctx context.Context, id string, req *RoutineRequest) (*Routine, error) {
	body := struct {
		Routine *RoutineRequest `json:"routine"`
	}{Routine: req}

	var resp struct {
		Routine []Routine `json:"routine"`
	}
	if err := c.doJSON(ctx, http.MethodPut, "/v1/routines/"+id, body, &resp); err != nil {
		return nil, fmt.Errorf("updating routine %s: %w", id, err)
	}
	if len(resp.Routine) == 0 {
		return nil, fmt.Errorf("updating routine %s: empty response", id)
	}
	return &resp.Routine[0], nil
}
