package openstack

import (
	"context"
	"fmt"
	"strings"

	"github.com/gophercloud/gophercloud/v2/openstack/identity/v3/roles"
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

func (c *Client) CreateSnapsentryUser(
	ctx context.Context,
	project string,
	projectId string,
	domainId string,
	role string,
	password string,
	updatePassword bool,
	prefix string,
) (users.User, string, error) {

	generatedUserName := fmt.Sprintf("%s-%s", prefix, strings.ToLower(project))
	fmt.Println(generatedUserName)
	requestedUser := users.User{}
	var requestID string

	if project == "" || projectId == "" || domainId == "" {
		return requestedUser, "", fmt.Errorf("Project Name, Project ID, Domain ID should all be provided. One of the values is empty.")
	}

	if role == "" {
		role = "member"
	}

	createOP := func(innerCtx context.Context) error {
		// Check if the user exists. Gopher SDK does not provide us option to get by name.
		existingUsers, err := c.ListSnapsentryUsers(innerCtx, prefix)
		if err != nil {
			return err
		}

		for _, user := range existingUsers {
			if user.Name != generatedUserName {
				continue
			}
			if !updatePassword {
				return fmt.Errorf("User (%s - %s) already exists. Skipping user creation. If you want to update password, please set update flag to true", user.Name, user.ID)
			}
			requestedUser = user
			_, err := users.Update(innerCtx, c.IdentityClient, requestedUser.ID, users.UpdateOpts{Password: password}).Extract()
			if err != nil {
				return err
			}
			break
		}
		if requestedUser.ID != "" {
			return nil
		}

		userCreationRequest := users.Create(innerCtx, c.IdentityClient, users.CreateOpts{
			Name:             generatedUserName,
			Description:      "Auto generated users for Snapsentry. Any modification to this user / password will impact snapsentry based snapshot operations",
			DefaultProjectID: projectId,
			DomainID:         domainId,
			Password:         password,
		})

		requestID = userCreationRequest.Header.Get("X-Openstack-Request-Id")
		user, err := userCreationRequest.Extract()
		if err != nil {
			return err
		}

		requestedUser = *user

		return nil

	}

	roleAddOP := func(innerCtx context.Context) error {

		requestedRole := roles.Role{}
		// Get all pages of roles
		listOpts := roles.ListOpts{}
		allPages, err := roles.List(c.IdentityClient, listOpts).AllPages(innerCtx)
		if err != nil {
			return fmt.Errorf("failed to list OpenStack roles: %w", err)
		}
		roleList, err := roles.ExtractRoles(allPages)

		for _, r := range roleList {
			if r.Name != role {
				continue
			}

			requestedRole = r
		}

		if requestedRole.ID == "" {
			return fmt.Errorf("Role (%s) does not exist.", requestedRole.Name)
		}

		roleAssignRequest := roles.Assign(innerCtx, c.IdentityClient, requestedRole.ID, roles.AssignOpts{
			UserID:    requestedUser.ID,
			ProjectID: projectId,
		})

		requestID += roleAssignRequest.Header.Get("X-Openstack-Request-Id")

		assignErr := roleAssignRequest.ExtractErr()
		if assignErr != nil {
			return assignErr
		}

		return nil
	}

	if err := c.executeWithRetry(ctx, "CreateUser", createOP); err != nil {
		return requestedUser, requestID, err
	}

	if err := c.executeWithRetry(ctx, "AssignRole", roleAddOP); err != nil {
		return requestedUser, requestID, err
	}

	return requestedUser, requestID, nil
}
