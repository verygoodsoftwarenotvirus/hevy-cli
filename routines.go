package hevy

import (
	"context"
	"encoding/json"
	"errors"
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

	var resp routineWriteResponse
	if err := c.doJSON(ctx, http.MethodPost, "/v1/routines", body, &resp); err != nil {
		return nil, fmt.Errorf("creating routine: %w", err)
	}
	routine, err := resp.decode()
	if err != nil {
		return nil, fmt.Errorf("creating routine: %w", err)
	}
	return routine, nil
}

// UpdateRoutine updates an existing routine.
func (c *Client) UpdateRoutine(ctx context.Context, id string, req *RoutineRequest) (*Routine, error) {
	body := struct {
		Routine *RoutineRequest `json:"routine"`
	}{Routine: req}

	var resp routineWriteResponse
	if err := c.doJSON(ctx, http.MethodPut, "/v1/routines/"+id, body, &resp); err != nil {
		return nil, fmt.Errorf("updating routine %s: %w", id, err)
	}
	routine, err := resp.decode()
	if err != nil {
		return nil, fmt.Errorf("updating routine %s: %w", id, err)
	}
	return routine, nil
}

// routineWriteResponse is the body Hevy answers a routine create or update with. Its
// routine has come back both as a one-element array and as a bare object, so it is
// decoded lazily to accept either.
type routineWriteResponse struct {
	Routine json.RawMessage `json:"routine"`
}

func (r *routineWriteResponse) decode() (*Routine, error) {
	if len(r.Routine) == 0 || string(r.Routine) == "null" {
		return nil, errors.New("empty response")
	}
	var list []Routine
	if err := json.Unmarshal(r.Routine, &list); err == nil {
		if len(list) == 0 {
			return nil, errors.New("empty response")
		}
		return &list[0], nil
	}
	var routine Routine
	if err := json.Unmarshal(r.Routine, &routine); err != nil {
		return nil, fmt.Errorf("decoding routine: %w", err)
	}
	return &routine, nil
}
