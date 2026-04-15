package hevy

import (
	"context"
	"net/http"
	"net/url"
	"time"
)

// GetExerciseHistory retrieves the exercise history for a given template ID.
// startDate and endDate are optional filters; pass nil to omit.
func (c *Client) GetExerciseHistory(ctx context.Context, exerciseTemplateID string, startDate, endDate *time.Time) ([]ExerciseHistoryEntry, error) {
	params := url.Values{}
	if startDate != nil {
		params.Set("start_date", startDate.UTC().Format(time.RFC3339))
	}
	if endDate != nil {
		params.Set("end_date", endDate.UTC().Format(time.RFC3339))
	}

	path := "/v1/exercise_history/" + exerciseTemplateID
	if encoded := params.Encode(); encoded != "" {
		path += "?" + encoded
	}

	var resp exerciseHistoryResponse
	if err := c.doJSON(ctx, http.MethodGet, path, nil, &resp); err != nil {
		return nil, err
	}
	return resp.ExerciseHistory, nil
}
