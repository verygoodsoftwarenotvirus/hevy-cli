package hevy

import (
	"context"
	"fmt"
	"iter"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

// GetWorkout retrieves a single workout by ID.
func (c *Client) GetWorkout(ctx context.Context, id string) (*Workout, error) {
	var w Workout
	if err := c.doJSON(ctx, http.MethodGet, "/v1/workouts/"+id, nil, &w); err != nil {
		return nil, err
	}
	return &w, nil
}

// ListWorkouts returns an iterator over all workouts, handling pagination automatically.
func (c *Client) ListWorkouts(ctx context.Context) iter.Seq2[Workout, error] {
	return fetchAllPages(ctx, func(ctx context.Context, page int) ([]Workout, int, error) {
		params := url.Values{}
		params.Set("page", strconv.Itoa(page))
		params.Set("pageSize", "10")

		var resp workoutsResponse
		if err := c.doJSON(ctx, http.MethodGet, "/v1/workouts?"+params.Encode(), nil, &resp); err != nil {
			return nil, 0, err
		}
		return resp.Workouts, resp.PageCount, nil
	})
}

// GetWorkoutCount returns the total number of workouts.
func (c *Client) GetWorkoutCount(ctx context.Context) (int, error) {
	var resp workoutCountResponse
	if err := c.doJSON(ctx, http.MethodGet, "/v1/workouts/count", nil, &resp); err != nil {
		return 0, err
	}
	return resp.WorkoutCount, nil
}

// ListWorkoutEvents returns an iterator over workout events (updates/deletes) since the given time.
func (c *Client) ListWorkoutEvents(ctx context.Context, since time.Time) iter.Seq2[WorkoutEvent, error] {
	return fetchAllPages(ctx, func(ctx context.Context, page int) ([]WorkoutEvent, int, error) {
		params := url.Values{}
		params.Set("page", strconv.Itoa(page))
		params.Set("pageSize", "10")
		params.Set("since", since.UTC().Format(time.RFC3339))

		var resp workoutEventsResponse
		if err := c.doJSON(ctx, http.MethodGet, "/v1/workouts/events?"+params.Encode(), nil, &resp); err != nil {
			return nil, 0, err
		}
		return resp.Events, resp.PageCount, nil
	})
}

// CreateWorkout creates a new workout.
func (c *Client) CreateWorkout(ctx context.Context, req *WorkoutRequest) (*Workout, error) {
	body := struct {
		Workout *WorkoutRequest `json:"workout"`
	}{Workout: req}

	var w Workout
	if err := c.doJSON(ctx, http.MethodPost, "/v1/workouts", body, &w); err != nil {
		return nil, fmt.Errorf("creating workout: %w", err)
	}
	return &w, nil
}

// UpdateWorkout updates an existing workout.
func (c *Client) UpdateWorkout(ctx context.Context, id string, req *WorkoutRequest) (*Workout, error) {
	body := struct {
		Workout *WorkoutRequest `json:"workout"`
	}{Workout: req}

	var w Workout
	if err := c.doJSON(ctx, http.MethodPut, "/v1/workouts/"+id, body, &w); err != nil {
		return nil, fmt.Errorf("updating workout %s: %w", id, err)
	}
	return &w, nil
}
