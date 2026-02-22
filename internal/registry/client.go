package registry

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/chrisdietr/coolify-patrol/pkg/types"
)

// Client handles Docker registry API calls
type Client struct {
	httpClient *http.Client
	userAgent  string
}

// DockerHubTagsResponse represents the Docker Hub API response
type DockerHubTagsResponse struct {
	Count    int                     `json:"count"`
	Next     string                  `json:"next"`
	Previous string                  `json:"previous"`
	Results  []DockerHubTag          `json:"results"`
}

// DockerHubTag represents a single tag from Docker Hub
type DockerHubTag struct {
	Name        string    `json:"name"`
	FullSize    int64     `json:"full_size"`
	LastUpdated time.Time `json:"last_updated"`
	Images      []struct {
		Digest string `json:"digest"`
	} `json:"images"`
}

// GHCRTagsResponse represents the GHCR API response  
type GHCRTagsResponse struct {
	Tags []GHCRTag `json:"tags"`
}

// GHCRTag represents a single tag from GHCR
type GHCRTag struct {
	Name   string `json:"name"`
	Digest string `json:"digest"`
}

// NewClient creates a new registry client
func NewClient() *Client {
	return &Client{
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		userAgent: "coolify-patrol/1.0",
	}
}

// GetTags fetches all tags for an image from the appropriate registry
func (c *Client) GetTags(ctx context.Context, image string) ([]types.RegistryTag, error) {
	if strings.HasPrefix(image, "ghcr.io/") {
		return c.getGHCRTags(ctx, image)
	}
	
	// Default to Docker Hub
	return c.getDockerHubTags(ctx, image)
}

// getDockerHubTags fetches tags from Docker Hub API v2
func (c *Client) getDockerHubTags(ctx context.Context, image string) ([]types.RegistryTag, error) {
	// Parse image name
	parts := strings.Split(image, "/")
	var namespace, repo string
	
	if len(parts) == 1 {
		// Official image (e.g., "postgres")
		namespace = "library"
		repo = parts[0]
	} else if len(parts) == 2 {
		// User image (e.g., "n8nio/n8n")
		namespace = parts[0]
		repo = parts[1]
	} else {
		return nil, fmt.Errorf("invalid Docker Hub image format: %s", image)
	}
	
	var allTags []types.RegistryTag
	url := fmt.Sprintf("https://registry.hub.docker.com/v2/repositories/%s/%s/tags/", namespace, repo)
	
	for url != "" {
		req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
		if err != nil {
			return nil, fmt.Errorf("creating request: %w", err)
		}
		
		req.Header.Set("User-Agent", c.userAgent)
		
		resp, err := c.httpClient.Do(req)
		if err != nil {
			return nil, fmt.Errorf("fetching tags: %w", err)
		}
		defer resp.Body.Close()
		
		if resp.StatusCode == 429 {
			// Rate limited - check retry header
			if retryAfter := resp.Header.Get("Retry-After"); retryAfter != "" {
				if seconds, err := strconv.Atoi(retryAfter); err == nil && seconds <= 300 {
					time.Sleep(time.Duration(seconds) * time.Second)
					continue // Retry the same URL
				}
			}
			return nil, fmt.Errorf("rate limited by Docker Hub")
		}
		
		if resp.StatusCode != 200 {
			return nil, fmt.Errorf("Docker Hub API returned status %d", resp.StatusCode)
		}
		
		var response DockerHubTagsResponse
		if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
			return nil, fmt.Errorf("decoding response: %w", err)
		}
		
		// Convert to our type
		for _, tag := range response.Results {
			registryTag := types.RegistryTag{
				Name: tag.Name,
			}
			if len(tag.Images) > 0 {
				registryTag.Digest = tag.Images[0].Digest
			}
			allTags = append(allTags, registryTag)
		}
		
		// Continue to next page if available
		url = response.Next
	}
	
	return allTags, nil
}

// getGHCRTags fetches tags from GitHub Container Registry
func (c *Client) getGHCRTags(ctx context.Context, image string) ([]types.RegistryTag, error) {
	// Remove ghcr.io/ prefix
	imagePath := strings.TrimPrefix(image, "ghcr.io/")
	
	// GHCR uses different API - we'll use the OCI distribution API
	url := fmt.Sprintf("https://ghcr.io/v2/%s/tags/list", imagePath)
	
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}
	
	req.Header.Set("User-Agent", c.userAgent)
	req.Header.Set("Accept", "application/vnd.oci.image.manifest.v1+json")
	
	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("fetching GHCR tags: %w", err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode == 429 {
		return nil, fmt.Errorf("rate limited by GHCR")
	}
	
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("GHCR API returned status %d", resp.StatusCode)
	}
	
	var response struct {
		Tags []string `json:"tags"`
	}
	
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("decoding GHCR response: %w", err)
	}
	
	// Convert to our type
	var tags []types.RegistryTag
	for _, tag := range response.Tags {
		tags = append(tags, types.RegistryTag{
			Name: tag,
			// GHCR doesn't provide digest in tags list, would need separate manifest calls
		})
	}
	
	return tags, nil
}

// GetLatestTag finds the latest tag from registry, filtering prereleases
func (c *Client) GetLatestTag(ctx context.Context, image string, excludePatterns []string) (string, error) {
	tags, err := c.GetTags(ctx, image)
	if err != nil {
		return "", err
	}
	
	if len(tags) == 0 {
		return "", fmt.Errorf("no tags found for image %s", image)
	}
	
	// Extract tag names
	var tagNames []string
	for _, tag := range tags {
		tagNames = append(tagNames, tag.Name)
	}
	
	// Filter prerelease tags
	filtered := filterTags(tagNames, excludePatterns)
	if len(filtered) == 0 {
		return "", fmt.Errorf("no stable tags found after filtering")
	}
	
	// Sort by semver if possible, otherwise lexicographically  
	sort.Slice(filtered, func(i, j int) bool {
		return compareVersions(filtered[i], filtered[j]) > 0
	})
	
	return filtered[0], nil
}

// filterTags removes tags matching exclude patterns
func filterTags(tags []string, excludePatterns []string) []string {
	var filtered []string
	
	for _, tag := range tags {
		excluded := false
		for _, pattern := range excludePatterns {
			if strings.Contains(tag, pattern) {
				excluded = true
				break
			}
		}
		if !excluded {
			filtered = append(filtered, tag)
		}
	}
	
	return filtered
}

// compareVersions compares two version strings, using semver when possible
func compareVersions(a, b string) int {
	// Try semver comparison first
	verA, errA := parseSimpleVersion(a)
	verB, errB := parseSimpleVersion(b)
	
	if errA == nil && errB == nil {
		if verA[0] != verB[0] { return verA[0] - verB[0] }
		if verA[1] != verB[1] { return verA[1] - verB[1] }
		if verA[2] != verB[2] { return verA[2] - verB[2] }
		return 0
	}
	
	// Fall back to lexicographic
	if a > b { return 1 }
	if a < b { return -1 }
	return 0
}

// parseSimpleVersion parses a simple x.y.z version
func parseSimpleVersion(v string) ([3]int, error) {
	// Remove 'v' prefix if present
	v = strings.TrimPrefix(v, "v")
	
	parts := strings.Split(v, ".")
	if len(parts) < 3 {
		return [3]int{}, fmt.Errorf("not a semver")
	}
	
	var version [3]int
	for i := 0; i < 3; i++ {
		// Handle pre-release suffixes by taking only the numeric part
		numPart := strings.FieldsFunc(parts[i], func(r rune) bool {
			return r < '0' || r > '9'
		})
		if len(numPart) == 0 {
			return [3]int{}, fmt.Errorf("not a semver")
		}
		
		num, err := strconv.Atoi(numPart[0])
		if err != nil {
			return [3]int{}, fmt.Errorf("not a semver")
		}
		version[i] = num
	}
	
	return version, nil
}