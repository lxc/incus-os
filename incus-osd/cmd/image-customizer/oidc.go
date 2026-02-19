package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"time"
)

func oidcZitadelQuery(ctx context.Context, endpoint string, reqStruct any, respStruct any) error {
	reqBody, err := json.Marshal(reqStruct)
	if err != nil {
		return err
	}

	r, err := http.NewRequestWithContext(ctx, http.MethodPost, os.Getenv("ZITADEL_URL")+"/"+endpoint, bytes.NewReader(reqBody))
	if err != nil {
		return err
	}

	r.Header.Add("Content-Type", "application/json")
	r.Header.Add("Accept", "application/json")
	r.Header.Add("Authorization", "Bearer "+os.Getenv("ZITADEL_TOKEN"))

	res, err := http.DefaultClient.Do(r)
	if err != nil {
		return err
	}

	defer res.Body.Close()

	err = json.NewDecoder(res.Body).Decode(&respStruct)
	if err != nil {
		return err
	}

	return nil
}

func oidcGetZitadelUser(ctx context.Context, userName string) (string, error) {
	// Prepare the request.
	type usersReq struct {
		Queries []map[string]map[string]string `json:"queries"`
	}

	req := usersReq{
		Queries: []map[string]map[string]string{
			{
				"userNameQuery": {
					"userName": userName,
				},
			},
		},
	}

	// Prepare the response.
	type usersResp struct {
		Result []struct {
			ID string `json:"userId"` //nolint:tagliatelle
		} `json:"result"`
	}

	resp := usersResp{}

	// Make the request.
	err := oidcZitadelQuery(ctx, "v2/users", &req, &resp)
	if err != nil {
		return "", err
	}

	// Process the response.
	if len(resp.Result) == 1 {
		return resp.Result[0].ID, nil
	}

	return "", nil
}

func oidcCreateZitadelUser(ctx context.Context, userName string) (string, error) {
	// Prepare the request.
	type usersReq struct {
		Organization string `json:"organizationId"` //nolint:tagliatelle
		Name         string `json:"username"`
		Human        struct {
			Profile struct {
				GivenName   string `json:"givenName"`   //nolint:tagliatelle
				FamilyName  string `json:"familyName"`  //nolint:tagliatelle
				DisplayName string `json:"displayName"` //nolint:tagliatelle
			} `json:"profile"`
			Email struct {
				Email    string `json:"email"`
				Verified bool   `json:"isVerified"` //nolint:tagliatelle
			} `json:"email"`
		} `json:"human"`
	}

	req := usersReq{
		Organization: os.Getenv("ZITADEL_ORGANIZATION"),
		Name:         userName,
	}
	req.Human.Profile.GivenName = userName
	req.Human.Profile.FamilyName = userName
	req.Human.Profile.DisplayName = userName
	req.Human.Email.Email = userName + "@" + "placeholder.arpa"
	req.Human.Email.Verified = true

	// Prepare the response.
	type usersResp struct {
		ID string `json:"id"`
	}

	resp := usersResp{}

	// Make the request.
	err := oidcZitadelQuery(ctx, "v2/users/new", &req, &resp)
	if err != nil {
		return "", err
	}

	// Process the response.
	if resp.ID == "" {
		return "", errors.New("failed to create Zitadel user")
	}

	return resp.ID, nil
}

func oidcGetZitadelProject(ctx context.Context, projectName string) (string, error) {
	// Prepare the request.
	type projectsReq struct {
		Filters []map[string]map[string]string `json:"filters"`
	}

	req := projectsReq{
		Filters: []map[string]map[string]string{
			{
				"projectNameFilter": {
					"projectName": projectName,
				},
			},
		},
	}

	// Prepare the response.
	type projectsResp struct {
		Projects []struct {
			ID string `json:"projectId"` //nolint:tagliatelle
		} `json:"projects"`
	}

	resp := projectsResp{}

	// Make the request.
	err := oidcZitadelQuery(ctx, "zitadel.project.v2.ProjectService/ListProjects", &req, &resp)
	if err != nil {
		return "", err
	}

	// Process the response.
	if len(resp.Projects) == 1 {
		return resp.Projects[0].ID, nil
	}

	return "", nil
}

func oidcCreateZitadelProject(ctx context.Context, projectName string) (string, error) {
	// Prepare the request.
	type projectsReq struct {
		Organization          string `json:"organizationId"` //nolint:tagliatelle
		Name                  string `json:"name"`
		AuthorizationRequired bool   `json:"authorizationRequired"` //nolint:tagliatelle
		ProjectAccessRequired bool   `json:"projectAccessRequired"` //nolint:tagliatelle
	}

	req := projectsReq{
		Organization:          os.Getenv("ZITADEL_ORGANIZATION"),
		Name:                  projectName,
		AuthorizationRequired: true,
		ProjectAccessRequired: true,
	}

	// Prepare the response.
	type projectsResp struct {
		ID string `json:"projectId"` //nolint:tagliatelle
	}

	resp := projectsResp{}

	// Make the request.
	err := oidcZitadelQuery(ctx, "zitadel.project.v2.ProjectService/CreateProject", &req, &resp)
	if err != nil {
		return "", err
	}

	// Process the response.
	if resp.ID == "" {
		return "", errors.New("failed to create Zitadel project")
	}

	return resp.ID, nil
}

func oidcGrantZitadelProject(ctx context.Context, projectID string, userID string) error {
	// Prepare the request.
	type authReq struct {
		Organization string `json:"organizationId"` //nolint:tagliatelle
		User         string `json:"userId"`         //nolint:tagliatelle
		Project      string `json:"projectId"`      //nolint:tagliatelle
	}

	req := authReq{
		Organization: os.Getenv("ZITADEL_ORGANIZATION"),
		User:         userID,
		Project:      projectID,
	}

	// Prepare the response.
	type authResp struct {
		ID string `json:"id"`
	}

	resp := authResp{}

	// Make the request.
	err := oidcZitadelQuery(ctx, "zitadel.authorization.v2.AuthorizationService/CreateAuthorization", &req, &resp)
	if err != nil {
		return err
	}

	// Process the response.
	if resp.ID == "" {
		return errors.New("failed to setup Zitadel authorization")
	}

	return nil
}

func oidcCreateZitadelApplication(ctx context.Context, projectID string, name string) (string, error) {
	// Prepare the request.
	type applicationReq struct {
		Project           string `json:"project_id"`
		Name              string `json:"name"`
		OIDCConfiguration struct {
			RedirectURIs    []string `json:"redirect_uris"`
			ResponseTypes   []string `json:"response_types"`
			GrantTypes      []string `json:"grant_types"`
			AppType         string   `json:"application_type"`
			AuthMethodType  string   `json:"auth_method_type"`
			DevMode         bool     `json:"development_mode"`
			AccessTokenType string   `json:"access_token_type"`
		} `json:"oidcConfiguration"` //nolint:tagliatelle
	}

	req := applicationReq{
		Project: projectID,
		Name:    name,
	}
	req.OIDCConfiguration.RedirectURIs = []string{"https://*/oidc/callback"}
	req.OIDCConfiguration.ResponseTypes = []string{"OIDC_RESPONSE_TYPE_CODE"}
	req.OIDCConfiguration.GrantTypes = []string{"OIDC_GRANT_TYPE_AUTHORIZATION_CODE", "OIDC_GRANT_TYPE_REFRESH_TOKEN", "OIDC_GRANT_TYPE_DEVICE_CODE"}
	req.OIDCConfiguration.AppType = "OIDC_APP_TYPE_USER_AGENT"
	req.OIDCConfiguration.AuthMethodType = "OIDC_AUTH_METHOD_TYPE_NONE"
	req.OIDCConfiguration.DevMode = true
	req.OIDCConfiguration.AccessTokenType = "OIDC_TOKEN_TYPE_JWT"

	// Prepare the response.
	type applicationResp struct {
		OIDCConfiguration struct {
			ClientID string `json:"clientId"` //nolint:tagliatelle
		} `json:"oidcConfiguration"` //nolint:tagliatelle
	}

	resp := applicationResp{}

	// Make the request.
	err := oidcZitadelQuery(ctx, "zitadel.application.v2.ApplicationService/CreateApplication", &req, &resp)
	if err != nil {
		return "", err
	}

	// Process the response.
	if resp.OIDCConfiguration.ClientID == "" {
		return "", errors.New("failed to setup Zitadel application")
	}

	return resp.OIDCConfiguration.ClientID, nil
}

func oidcGenerate(ctx context.Context, userName string) (string, string, error) {
	// Validate that we have support for this feature.
	for _, env := range []string{"DISCOURSE_URL", "ZITADEL_ORGANIZATION", "ZITADEL_URL", "ZITADEL_TOKEN"} {
		if os.Getenv(env) == "" {
			slog.Error("Requested OIDC login but missing environment variable", "variable", env)

			return "", "", errors.New("OIDC authentication isn't supported at the moment")
		}
	}

	// Check that the Discourse account is valid.
	userName, err := oidcCheckDiscourse(ctx, userName)
	if err != nil {
		slog.Warn("Requested OIDC login for invalid forum account", "username", userName)

		return "", "", fmt.Errorf("failed OIDC validation: %w", err)
	}

	// Check for an existing Zitadel user.
	userID, err := oidcGetZitadelUser(ctx, userName)
	if err != nil {
		slog.Error("Failed getting Zitadel user", "name", userName)

		return "", "", fmt.Errorf("failed checking for OIDC user: %w", err)
	}

	// Create a new Zitadel user if missing.
	if userID == "" {
		// Create the user.
		userID, err = oidcCreateZitadelUser(ctx, userName)
		if err != nil {
			slog.Error("Failed creating Zitadel user", "name", userName)

			return "", "", fmt.Errorf("failed creating OIDC user: %w", err)
		}
	}

	// Check for an existing Zitadel Project.
	projectName := "CL - " + userName

	projectID, err := oidcGetZitadelProject(ctx, projectName)
	if err != nil {
		slog.Error("Failed getting Zitadel project", "name", projectName)

		return "", "", fmt.Errorf("failed checking for OIDC project: %w", err)
	}

	// Create a new Zitadel Project if missing.
	if projectID == "" {
		// Create the project.
		projectID, err = oidcCreateZitadelProject(ctx, projectName)
		if err != nil {
			slog.Error("Failed creating Zitadel project", "name", projectName)

			return "", "", fmt.Errorf("failed creating OIDC project: %w", err)
		}

		// Grant access to the project.
		err = oidcGrantZitadelProject(ctx, projectID, userID)
		if err != nil {
			slog.Error("Failed setting up Zitadel authorization", "user_id", userID, "project_id", projectID)

			return "", "", fmt.Errorf("failed granting OIDC project access: %w", err)
		}
	}

	// Register a new application.
	applicationName := time.Now().UTC().Format("20060102-150405")

	clientID, err := oidcCreateZitadelApplication(ctx, projectID, applicationName)
	if err != nil {
		slog.Error("Failed creating Zitadel application", "username", userName)

		return "", "", fmt.Errorf("failed creating OIDC application: %w", err)
	}

	// Return the ID.
	return os.Getenv("ZITADEL_URL"), clientID, nil
}

func oidcCheckDiscourse(ctx context.Context, userName string) (string, error) {
	// Check that the username is valid.
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, os.Getenv("DISCOURSE_URL")+"/u/"+userName+".json", nil)
	if err != nil {
		return "", err
	}

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}

	defer func() { _ = res.Body.Close() }()

	if res.StatusCode != http.StatusOK {
		return "", errors.New("forum account doesn't exist")
	}

	// Get exact spelling from the response.
	type userResp struct {
		User struct {
			Username string `json:"username"`
		} `json:"user"`
	}

	resp := userResp{}

	err = json.NewDecoder(res.Body).Decode(&resp)
	if err != nil {
		return "", err
	}

	if resp.User.Username == "" {
		return "", errors.New("forum account data is broken")
	}

	return resp.User.Username, nil
}
