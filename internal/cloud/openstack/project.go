package openstack

import (
	"context"

	"github.com/gophercloud/gophercloud/v2/openstack/identity/v3/projects"
)

func (c *Client) ListSubscribedProjects(ctx context.Context) ([]projects.Project, error) {

	projectResult := []projects.Project{}

	listOP := func(innerCtx context.Context) error {
		opts := projects.ListOpts{
			Tags: "snapsentry-enabled",
		}

		resp, err := projects.List(c.IdentityClient, opts).AllPages(innerCtx)
		if err != nil {
			return err
		}

		projects, err := projects.ExtractProjects(resp)
		if err != nil {
			return err
		}

		projectResult = projects
		return nil
	}

	if err := c.executeWithRetry(ctx, "ListProject", listOP); err != nil {
		return projectResult, err
	}

	return projectResult, nil
}
