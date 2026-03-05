package openstack

import (
	"context"

	"github.com/gophercloud/gophercloud/v2/openstack/identity/v3/users"
)

func (c *Client) ListSnapsentryUsers(ctx context.Context, prefix string) ([]users.User, error) {

	if prefix == "" {
		prefix = "snapsentry-"
	}

	userListResult := []users.User{}

	listOP := func(innerCtx context.Context) error {

		opts := users.ListOpts{
			Filters: map[string]string{
				"name__contains": prefix,
			},
		}

		resp, err := users.List(c.IdentityClient, opts).AllPages(innerCtx)
		if err != nil {
			return err
		}

		users, err := users.ExtractUsers(resp)
		if err != nil {
			return err
		}

		userListResult = users
		return nil
	}

	if err := c.executeWithRetry(ctx, "ListUsers", listOP); err != nil {
		return userListResult, err
	}

	return userListResult, nil
}
