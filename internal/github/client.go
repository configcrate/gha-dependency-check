package github

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/configcrate/gha-dependency-check/internal/model"
)

type Client struct {
	baseURL    string
	token      string
	httpClient *http.Client
}

type repositoryResponse struct {
	Archived bool `json:"archived"`
	Disabled bool `json:"disabled"`
}

type errorResponse struct {
	Message string `json:"message"`
}

func NewClient(baseURL string, timeout time.Duration) *Client {
	if baseURL == "" {
		baseURL = os.Getenv("GITHUB_API_URL")
	}
	if baseURL == "" {
		baseURL = "https://api.github.com"
	}
	token := os.Getenv("GH_TOKEN")
	if token == "" {
		token = os.Getenv("GITHUB_TOKEN")
	}
	return &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		token:   token,
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}
}

func (client *Client) Check(ctx context.Context, dependency model.Dependency) model.Result {
	repositoryURL := fmt.Sprintf(
		"%s/repos/%s/%s",
		client.baseURL,
		url.PathEscape(dependency.Owner),
		url.PathEscape(dependency.Repo),
	)
	status, body, headers, err := client.get(ctx, repositoryURL)
	if err != nil {
		return apiError(dependency, 0, err.Error())
	}
	if status != http.StatusOK {
		return client.repositoryFailure(dependency, status, body, headers)
	}

	var repository repositoryResponse
	if err := json.Unmarshal(body, &repository); err != nil {
		return apiError(dependency, status, "GitHub returned an invalid repository response")
	}
	if repository.Disabled {
		return result(dependency, model.StatusDisabled, status, "repository is disabled")
	}
	if repository.Archived {
		return result(dependency, model.StatusArchived, status, "repository is archived and read-only")
	}

	commitURL := fmt.Sprintf(
		"%s/commits/%s",
		repositoryURL,
		url.PathEscape(dependency.Ref),
	)
	status, body, headers, err = client.get(ctx, commitURL)
	if err != nil {
		return apiError(dependency, 0, err.Error())
	}
	if status == http.StatusNotFound || status == http.StatusUnprocessableEntity {
		return result(dependency, model.StatusMissingRef, status, "referenced commit, tag, or branch does not exist")
	}
	if status != http.StatusOK {
		return client.referenceFailure(dependency, status, body, headers)
	}

	if dependency.Path != "" {
		contentURL := fmt.Sprintf(
			"%s/contents/%s?ref=%s",
			repositoryURL,
			escapePath(dependency.Path),
			url.QueryEscape(dependency.Ref),
		)
		status, body, headers, err = client.get(ctx, contentURL)
		if err != nil {
			return apiError(dependency, 0, err.Error())
		}
		if status == http.StatusNotFound {
			return result(dependency, model.StatusMissingPath, status, "action or reusable workflow path does not exist at the referenced ref")
		}
		if status != http.StatusOK {
			return client.referenceFailure(dependency, status, body, headers)
		}
	}

	return result(dependency, model.StatusHealthy, http.StatusOK, "repository, ref, and path are available")
}

func (client *Client) repositoryFailure(
	dependency model.Dependency,
	status int,
	body []byte,
	headers http.Header,
) model.Result {
	message := responseMessage(body)
	if rateLimited(status, headers) {
		return apiError(dependency, status, "GitHub API rate limit exhausted")
	}
	if status == http.StatusForbidden && strings.EqualFold(message, "Repository access blocked") {
		return result(dependency, model.StatusBlocked, status, message)
	}
	if status == http.StatusNotFound {
		return result(
			dependency,
			model.StatusUnavailable,
			status,
			"repository does not exist or is not accessible to the current token",
		)
	}
	if status == http.StatusUnavailableForLegalReasons {
		return result(dependency, model.StatusBlocked, status, fallback(message, "repository is unavailable for legal reasons"))
	}
	return apiError(dependency, status, fallback(message, fmt.Sprintf("unexpected GitHub API status %d", status)))
}

func (client *Client) referenceFailure(
	dependency model.Dependency,
	status int,
	body []byte,
	headers http.Header,
) model.Result {
	if rateLimited(status, headers) {
		return apiError(dependency, status, "GitHub API rate limit exhausted")
	}
	return apiError(
		dependency,
		status,
		fallback(responseMessage(body), fmt.Sprintf("unexpected GitHub API status %d", status)),
	)
}

func (client *Client) get(ctx context.Context, endpoint string) (int, []byte, http.Header, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return 0, nil, nil, err
	}
	request.Header.Set("Accept", "application/vnd.github+json")
	request.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	request.Header.Set("User-Agent", "configcrate-gha-dependency-check")
	if client.token != "" {
		request.Header.Set("Authorization", "Bearer "+client.token)
	}

	response, err := client.httpClient.Do(request)
	if err != nil {
		return 0, nil, nil, fmt.Errorf("GitHub API request failed: %w", err)
	}
	defer response.Body.Close()
	body, err := io.ReadAll(io.LimitReader(response.Body, 1<<20))
	if err != nil {
		return response.StatusCode, nil, response.Header, fmt.Errorf("read GitHub API response: %w", err)
	}
	return response.StatusCode, body, response.Header, nil
}

func responseMessage(body []byte) string {
	var response errorResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return ""
	}
	return strings.TrimSpace(response.Message)
}

func rateLimited(status int, headers http.Header) bool {
	return status == http.StatusForbidden && headers.Get("X-RateLimit-Remaining") == "0"
}

func escapePath(path string) string {
	parts := strings.Split(path, "/")
	for index, part := range parts {
		parts[index] = url.PathEscape(part)
	}
	return strings.Join(parts, "/")
}

func fallback(value string, defaultValue string) string {
	if strings.TrimSpace(value) == "" {
		return defaultValue
	}
	return value
}

func result(
	dependency model.Dependency,
	status model.Status,
	httpStatus int,
	message string,
) model.Result {
	return model.Result{
		Dependency: dependency,
		Status:     status,
		HTTPStatus: httpStatus,
		Message:    message,
	}
}

func apiError(dependency model.Dependency, httpStatus int, message string) model.Result {
	return result(dependency, model.StatusAPIError, httpStatus, message)
}
