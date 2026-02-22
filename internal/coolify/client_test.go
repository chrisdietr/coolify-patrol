package coolify

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestNewClient(t *testing.T) {
	baseURL := "http://localhost:8000"
	token := "test-token"
	
	client := NewClient(baseURL, token)
	
	if client.baseURL != baseURL {
		t.Errorf("expected baseURL '%s', got '%s'", baseURL, client.baseURL)
	}
	
	if client.token != token {
		t.Errorf("expected token '%s', got '%s'", token, client.token)
	}
	
	if client.httpClient.Timeout != 30*time.Second {
		t.Errorf("expected timeout 30s, got %v", client.httpClient.Timeout)
	}
}

func TestNewClientTrimsSlash(t *testing.T) {
	baseURL := "http://localhost:8000/"
	client := NewClient(baseURL, "test-token")
	
	expected := "http://localhost:8000"
	if client.baseURL != expected {
		t.Errorf("expected baseURL '%s', got '%s'", expected, client.baseURL)
	}
}

func TestListApplications(t *testing.T) {
	// Mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/v1/applications" {
			t.Errorf("expected path '/api/v1/applications', got '%s'", r.URL.Path)
		}
		
		if r.Method != http.MethodGet {
			t.Errorf("expected GET method, got '%s'", r.Method)
		}
		
		auth := r.Header.Get("Authorization")
		if auth != "Bearer test-token" {
			t.Errorf("expected 'Bearer test-token', got '%s'", auth)
		}
		
		response := ApplicationsListResponse{
			Data: []ApplicationResponse{
				{
					UUID:        "app-1",
					Name:        "test-app",
					DockerImage: "nginx:1.21",
					Status:      "running",
				},
				{
					UUID:        "app-2",
					Name:        "postgres",
					DockerImage: "postgres:13.4",
					Status:      "running",
				},
			},
		}
		
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()
	
	client := NewClient(server.URL, "test-token")
	
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	apps, err := client.ListApplications(ctx)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	
	if len(apps) != 2 {
		t.Errorf("expected 2 apps, got %d", len(apps))
		return
	}
	
	// Check first app
	if apps[0].UUID != "app-1" {
		t.Errorf("expected UUID 'app-1', got '%s'", apps[0].UUID)
	}
	if apps[0].Name != "test-app" {
		t.Errorf("expected name 'test-app', got '%s'", apps[0].Name)
	}
	if apps[0].DockerImage != "nginx:1.21" {
		t.Errorf("expected image 'nginx:1.21', got '%s'", apps[0].DockerImage)
	}
}

func TestGetApplication(t *testing.T) {
	// Mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		expectedPath := "/api/v1/applications/test-uuid"
		if r.URL.Path != expectedPath {
			t.Errorf("expected path '%s', got '%s'", expectedPath, r.URL.Path)
		}
		
		response := ApplicationResponse{
			UUID:        "test-uuid",
			Name:        "test-app",
			DockerImage: "nginx:1.21",
			Status:      "running",
		}
		
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()
	
	client := NewClient(server.URL, "test-token")
	
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	app, err := client.GetApplication(ctx, "test-uuid")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	
	if app.UUID != "test-uuid" {
		t.Errorf("expected UUID 'test-uuid', got '%s'", app.UUID)
	}
	if app.Name != "test-app" {
		t.Errorf("expected name 'test-app', got '%s'", app.Name)
	}
}

func TestGetApplicationNotFound(t *testing.T) {
	// Mock server returning 404
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))
	defer server.Close()
	
	client := NewClient(server.URL, "test-token")
	
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	_, err := client.GetApplication(ctx, "non-existent")
	if err == nil {
		t.Error("expected error for non-existent app, got nil")
	}
	
	if err.Error() != "application not found: non-existent" {
		t.Errorf("unexpected error message: %v", err)
	}
}

func TestUpdateApplication(t *testing.T) {
	// Mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		expectedPath := "/api/v1/applications/test-uuid"
		if r.URL.Path != expectedPath {
			t.Errorf("expected path '%s', got '%s'", expectedPath, r.URL.Path)
		}
		
		if r.Method != http.MethodPatch {
			t.Errorf("expected PATCH method, got '%s'", r.Method)
		}
		
		var updateReq UpdateRequest
		if err := json.NewDecoder(r.Body).Decode(&updateReq); err != nil {
			t.Errorf("failed to decode request body: %v", err)
			return
		}
		
		if updateReq.DockerImage != "nginx:1.22" {
			t.Errorf("expected image 'nginx:1.22', got '%s'", updateReq.DockerImage)
		}
		
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()
	
	client := NewClient(server.URL, "test-token")
	
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	err := client.UpdateApplication(ctx, "test-uuid", "nginx:1.22")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRestartApplication(t *testing.T) {
	// Mock server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		expectedPath := "/api/v1/applications/test-uuid/restart"
		if r.URL.Path != expectedPath {
			t.Errorf("expected path '%s', got '%s'", expectedPath, r.URL.Path)
		}
		
		if r.Method != http.MethodPost {
			t.Errorf("expected POST method, got '%s'", r.Method)
		}
		
		w.WriteHeader(http.StatusAccepted)
	}))
	defer server.Close()
	
	client := NewClient(server.URL, "test-token")
	
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	
	err := client.RestartApplication(ctx, "test-uuid")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestExtractImageAndTag(t *testing.T) {
	tests := []struct {
		input         string
		expectedImage string
		expectedTag   string
	}{
		{
			input:         "nginx:1.21",
			expectedImage: "nginx",
			expectedTag:   "1.21",
		},
		{
			input:         "ghcr.io/owner/repo:v1.2.3",
			expectedImage: "ghcr.io/owner/repo",
			expectedTag:   "v1.2.3",
		},
		{
			input:         "postgres",
			expectedImage: "postgres",
			expectedTag:   "latest",
		},
		{
			input:         "registry.example.com:5000/myapp:v2.0.0",
			expectedImage: "registry.example.com:5000/myapp",
			expectedTag:   "v2.0.0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			image, tag := ExtractImageAndTag(tt.input)
			
			if image != tt.expectedImage {
				t.Errorf("expected image '%s', got '%s'", tt.expectedImage, image)
			}
			
			if tag != tt.expectedTag {
				t.Errorf("expected tag '%s', got '%s'", tt.expectedTag, tag)
			}
		})
	}
}

func TestBuildImageReference(t *testing.T) {
	tests := []struct {
		image    string
		tag      string
		expected string
	}{
		{"nginx", "1.21", "nginx:1.21"},
		{"ghcr.io/owner/repo", "v1.2.3", "ghcr.io/owner/repo:v1.2.3"},
		{"postgres", "latest", "postgres:latest"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			result := BuildImageReference(tt.image, tt.tag)
			if result != tt.expected {
				t.Errorf("expected '%s', got '%s'", tt.expected, result)
			}
		})
	}
}

func TestTestConnection(t *testing.T) {
	tests := []struct {
		name         string
		statusCode   int
		expectError  bool
		errorMessage string
	}{
		{
			name:        "successful connection",
			statusCode:  http.StatusOK,
			expectError: false,
		},
		{
			name:         "unauthorized",
			statusCode:   http.StatusUnauthorized,
			expectError:  true,
			errorMessage: "authentication failed - check your API token",
		},
		{
			name:         "other error status",
			statusCode:   http.StatusInternalServerError,
			expectError:  true,
			errorMessage: "unexpected status code: 500",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tt.statusCode)
				if tt.statusCode == http.StatusOK {
					json.NewEncoder(w).Encode(ApplicationsListResponse{})
				}
			}))
			defer server.Close()
			
			client := NewClient(server.URL, "test-token")
			
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			
			err := client.TestConnection(ctx)
			
			if tt.expectError {
				if err == nil {
					t.Error("expected error, got nil")
					return
				}
				
				if tt.errorMessage != "" && err.Error() != tt.errorMessage {
					t.Errorf("expected error '%s', got '%s'", tt.errorMessage, err.Error())
				}
			} else {
				if err != nil {
					t.Errorf("unexpected error: %v", err)
				}
			}
		})
	}
}