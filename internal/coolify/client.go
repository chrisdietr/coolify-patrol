package coolify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/chrisdietr/coolify-patrol/pkg/types"
)

// Client handles Coolify API calls
type Client struct {
	baseURL    string
	token      string
	httpClient *http.Client
}

// ApplicationResponse represents Coolify's application response
type ApplicationResponse struct {
	UUID        string `json:"uuid"`
	Name        string `json:"name"`
	DockerImage string `json:"docker_image"`
	Status      string `json:"status"`
}

// ApplicationsListResponse represents the response from listing applications
type ApplicationsListResponse struct {
	Data []ApplicationResponse `json:"data"`
}

// UpdateRequest represents an application update request
type UpdateRequest struct {
	DockerImage string `json:"docker_image"`
}

// NewClient creates a new Coolify API client
func NewClient(baseURL, token string) *Client {
	// Ensure baseURL doesn't end with slash
	baseURL = strings.TrimSuffix(baseURL, "/")
	
	return &Client{
		baseURL: baseURL,
		token:   token,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// ListApplications retrieves all applications from Coolify
func (c *Client) ListApplications(ctx context.Context) ([]types.CoolifyApplication, error) {
	url := fmt.Sprintf("%s/api/v1/applications", c.baseURL)
	
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("making request: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Coolify API returned status %d", resp.StatusCode)
	}
	
	var response ApplicationsListResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}
	
	// Convert to our types
	var applications []types.CoolifyApplication
	for _, app := range response.Data {
		applications = append(applications, types.CoolifyApplication{
			UUID:        app.UUID,
			Name:        app.Name,
			DockerImage: app.DockerImage,
			Status:      app.Status,
		})
	}
	
	return applications, nil
}

// GetApplication retrieves a specific application by UUID
func (c *Client) GetApplication(ctx context.Context, uuid string) (*types.CoolifyApplication, error) {
	url := fmt.Sprintf("%s/api/v1/applications/%s", c.baseURL, uuid)
	
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/json")
	
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("making request: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode == http.StatusNotFound {
		return nil, fmt.Errorf("application not found: %s", uuid)
	}
	
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Coolify API returned status %d", resp.StatusCode)
	}
	
	var app ApplicationResponse
	if err := json.NewDecoder(resp.Body).Decode(&app); err != nil {
		return nil, fmt.Errorf("decoding response: %w", err)
	}
	
	return &types.CoolifyApplication{
		UUID:        app.UUID,
		Name:        app.Name,
		DockerImage: app.DockerImage,
		Status:      app.Status,
	}, nil
}

// UpdateApplication updates an application's Docker image
func (c *Client) UpdateApplication(ctx context.Context, uuid, newImage string) error {
	url := fmt.Sprintf("%s/api/v1/applications/%s", c.baseURL, uuid)
	
	updateReq := UpdateRequest{
		DockerImage: newImage,
	}
	
	jsonData, err := json.Marshal(updateReq)
	if err != nil {
		return fmt.Errorf("marshaling update request: %w", err)
	}
	
	req, err := http.NewRequestWithContext(ctx, "PATCH", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}
	
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("making request: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode == http.StatusNotFound {
		return fmt.Errorf("application not found: %s", uuid)
	}
	
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent {
		return fmt.Errorf("Coolify API returned status %d", resp.StatusCode)
	}
	
	return nil
}

// RestartApplication triggers a restart/redeploy of an application
func (c *Client) RestartApplication(ctx context.Context, uuid string) error {
	url := fmt.Sprintf("%s/api/v1/applications/%s/restart", c.baseURL, uuid)
	
	req, err := http.NewRequestWithContext(ctx, "POST", url, nil)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}
	
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Content-Type", "application/json")
	
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("making request: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode == http.StatusNotFound {
		return fmt.Errorf("application not found: %s", uuid)
	}
	
	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusNoContent && resp.StatusCode != http.StatusAccepted {
		return fmt.Errorf("Coolify API returned status %d", resp.StatusCode)
	}
	
	return nil
}

// ExtractImageAndTag splits a Docker image reference into image and tag parts
func ExtractImageAndTag(dockerImage string) (string, string) {
	// Handle cases like:
	// postgres:17.2 -> postgres, 17.2
	// ghcr.io/owner/repo:v1.2.3 -> ghcr.io/owner/repo, v1.2.3
	// registry.example.com:5000/myapp:v2.0.0 -> registry.example.com:5000/myapp, v2.0.0
	// nginx -> nginx, latest
	
	// Find the last colon to handle registry URLs with ports
	lastColon := strings.LastIndex(dockerImage, ":")
	if lastColon == -1 {
		return dockerImage, "latest"
	}
	
	// Check if this colon is part of a port number (registry:port/image format)
	// If there's a slash after the colon, it's likely a port
	afterColon := dockerImage[lastColon+1:]
	if strings.Contains(afterColon, "/") {
		return dockerImage, "latest"
	}
	
	return dockerImage[:lastColon], afterColon
}

// BuildImageReference combines an image name and tag into a full Docker image reference
func BuildImageReference(image, tag string) string {
	return fmt.Sprintf("%s:%s", image, tag)
}

// TestConnection tests the connection to Coolify API
func (c *Client) TestConnection(ctx context.Context) error {
	url := fmt.Sprintf("%s/api/v1/applications", c.baseURL)
	
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return fmt.Errorf("creating request: %w", err)
	}
	
	req.Header.Set("Authorization", "Bearer "+c.token)
	req.Header.Set("Accept", "application/json")
	
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("making request: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode == http.StatusUnauthorized {
		return fmt.Errorf("authentication failed - check your API token")
	}
	
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}
	
	return nil
}