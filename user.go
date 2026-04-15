package hevy

import (
	"context"
	"net/http"
)

// GetUserInfo retrieves the authenticated user's information.
func (c *Client) GetUserInfo(ctx context.Context) (*UserInfo, error) {
	var resp userInfoResponse
	if err := c.doJSON(ctx, http.MethodGet, "/v1/user/info", nil, &resp); err != nil {
		return nil, err
	}
	return &resp.Data, nil
}
