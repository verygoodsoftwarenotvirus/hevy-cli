package hevy

import (
	"context"
	"fmt"
	"iter"
	"net/http"
	"net/url"
	"strconv"
)

// GetRoutineFolder retrieves a single routine folder by ID.
func (c *Client) GetRoutineFolder(ctx context.Context, id int) (*RoutineFolder, error) {
	var f RoutineFolder
	if err := c.doJSON(ctx, http.MethodGet, fmt.Sprintf("/v1/routine_folders/%d", id), nil, &f); err != nil {
		return nil, err
	}
	return &f, nil
}

// ListRoutineFolders returns an iterator over all routine folders, handling pagination automatically.
func (c *Client) ListRoutineFolders(ctx context.Context) iter.Seq2[RoutineFolder, error] {
	return fetchAllPages(ctx, func(ctx context.Context, page int) ([]RoutineFolder, int, error) {
		params := url.Values{}
		params.Set("page", strconv.Itoa(page))
		params.Set("pageSize", "10")

		var resp routineFoldersResponse
		if err := c.doJSON(ctx, http.MethodGet, "/v1/routine_folders?"+params.Encode(), nil, &resp); err != nil {
			return nil, 0, err
		}
		return resp.RoutineFolders, resp.PageCount, nil
	})
}

// CreateRoutineFolder creates a new routine folder.
func (c *Client) CreateRoutineFolder(ctx context.Context, req *RoutineFolderRequest) (*RoutineFolder, error) {
	body := struct {
		RoutineFolder *RoutineFolderRequest `json:"routine_folder"`
	}{RoutineFolder: req}

	var resp struct {
		RoutineFolder RoutineFolder `json:"routine_folder"`
	}
	if err := c.doJSON(ctx, http.MethodPost, "/v1/routine_folders", body, &resp); err != nil {
		return nil, fmt.Errorf("creating routine folder: %w", err)
	}
	return &resp.RoutineFolder, nil
}
