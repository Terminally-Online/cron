package endpoint

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"
)

func TestAPI(t *testing.T) {
	tmpDB := "test_api.db"
	defer os.Remove(tmpDB)

	handler, err := NewEndpointHandler(tmpDB, 10)
	if err != nil {
		t.Fatalf("Failed to create handler: %v", err)
	}
	defer handler.Close()

	api := NewAPI(handler)

	testEndpoint := EndpointRequest{
		URL:     "https://test.com",
		Method:  "GET",
		Timeout: time.Second,
		Status:  http.StatusOK,
	}

	for i := 0; i < 3; i++ {
		response := EndpointResponse{
			Endpoint:  testEndpoint,
			Status:    http.StatusOK,
			Timestamp: time.Now().Add(-time.Duration(i) * time.Hour),
			Duration:  100 * time.Millisecond,
		}
		if err := handler.storeResponse(response); err != nil {
			t.Fatalf("Failed to store test response: %v", err)
		}
	}

	tests := []struct {
		name           string
		path           string
		expectedStatus int
		validateBody   func(t *testing.T, body []byte)
	}{
		{
			name:           "get endpoints list",
			path:           "/endpoints",
			expectedStatus: http.StatusOK,
			validateBody: func(t *testing.T, body []byte) {
				var response EndpointListResponse
				if err := json.Unmarshal(body, &response); err != nil {
					t.Fatalf("Failed to unmarshal response: %v", err)
				}
				if len(response.URLs) != 1 {
					t.Errorf("Expected 1 endpoint, got %d", len(response.URLs))
				}
				if response.URLs[0] != "https://test.com" {
					t.Errorf("Expected URL https://test.com, got %s", response.URLs[0])
				}
			},
		},
		{
			name:           "get endpoint history",
			path:           "/endpoint/history?url=https://test.com",
			expectedStatus: http.StatusOK,
			validateBody: func(t *testing.T, body []byte) {
				var response HistoryResponse
				if err := json.Unmarshal(body, &response); err != nil {
					t.Fatalf("Failed to unmarshal response: %v", err)
				}
				if len(response.History) != 3 {
					t.Errorf("Expected 3 history entries, got %d", len(response.History))
				}
				if response.Stats.TotalChecks != 3 {
					t.Errorf("Expected 3 total checks, got %d", response.Stats.TotalChecks)
				}
			},
		},
		{
			name:           "get endpoint history - missing url",
			path:           "/endpoint/history",
			expectedStatus: http.StatusBadRequest,
		},
		{
			name:           "get endpoint history - not found",
			path:           "/endpoint/history?url=https://nonexistent.com",
			expectedStatus: http.StatusNotFound,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest("GET", tt.path, nil)
			rr := httptest.NewRecorder()

			api.ServeHTTP(rr, req)

			if rr.Code != tt.expectedStatus {
				t.Errorf("Expected status %d, got %d", tt.expectedStatus, rr.Code)
			}

			if tt.validateBody != nil && rr.Code == http.StatusOK {
				tt.validateBody(t, rr.Body.Bytes())
			}
		})
	}
}
