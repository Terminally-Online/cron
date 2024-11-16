package endpoint

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"

	"go.etcd.io/bbolt"
)

func TestEndpointHandler(t *testing.T) {
	tmpDB := "test.db"
	defer os.Remove(tmpDB)

	handler, err := NewEndpointHandler(tmpDB, 10)
	if err != nil {
		t.Fatalf("Failed to create handler: %v", err)
	}
	defer handler.Close()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/success":
			w.WriteHeader(http.StatusOK)
		case "/error":
			w.WriteHeader(http.StatusInternalServerError)
		case "/timeout":
			time.Sleep(2 * time.Second)
			w.WriteHeader(http.StatusOK)
		case "/content":
			w.WriteHeader(http.StatusOK)
			w.Write([]byte("this is the expected text content"))
		}
	}))
	defer server.Close()

	if err := handler.db.Update(func(tx *bbolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte(endpointBucket))
		return err
	}); err != nil {
		t.Fatalf("Failed to create bucket: %v", err)
	}

	tests := []struct {
		name     string
		request  EndpointRequest
		wantErr  bool
		wantCode int
	}{
		{
			name: "successful request",
			request: EndpointRequest{
				URL:           server.URL + "/success",
				Method:        "GET",
				Timeout:       time.Second,
				Status:        http.StatusOK,
				RetryAttempts: 1,
			},
			wantErr:  false,
			wantCode: http.StatusOK,
		},
		{
			name: "retry on error",
			request: EndpointRequest{
				URL:           server.URL + "/error",
				Method:        "GET",
				Timeout:       time.Second,
				Status:        http.StatusInternalServerError,
				RetryAttempts: 2,
				RetryDelay:    100 * time.Millisecond,
			},
			wantErr:  true,
			wantCode: http.StatusInternalServerError,
		},
		{
			name: "timeout",
			request: EndpointRequest{
				URL:           server.URL + "/timeout",
				Method:        "GET",
				Timeout:       time.Second,
				Status:        http.StatusOK,
				RetryAttempts: 1,
			},
			wantErr:  true,
			wantCode: 0,
		},
		{
			name: "content verification",
			request: EndpointRequest{
				URL:             server.URL + "/content",
				Method:          "GET",
				Timeout:         time.Second,
				Status:          http.StatusOK,
				RetryAttempts:   1,
				ExpectedContent: "expected text",
			},
			wantErr:  false,
			wantCode: http.StatusOK,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			if err := handler.db.Update(func(tx *bbolt.Tx) error {
				b := tx.Bucket([]byte(endpointBucket))
				return b.Delete([]byte(tt.request.URL))
			}); err != nil {
				t.Fatalf("Failed to clear previous entries: %v", err)
			}

			resp := handler.Handle(context.Background(), tt.request)

			if (resp.Error != nil) != tt.wantErr {
				t.Errorf("Handle() error = %v, wantErr %v", resp.Error, tt.wantErr)
			}

			if resp.Status != tt.wantCode {
				t.Errorf("Handle() status = %v, want %v", resp.Status, tt.wantCode)
			}

			time.Sleep(100 * time.Millisecond)

			history, err := handler.GetEndpointHistory(tt.request.URL)
			if err != nil {
				t.Errorf("Failed to get history: %v", err)
			}
			if len(history) == 0 {
				t.Error("No history entry created")

				if err := handler.db.View(func(tx *bbolt.Tx) error {
					b := tx.Bucket([]byte(endpointBucket))
					if b == nil {
						t.Log("Bucket does not exist")
						return nil
					}
					return b.ForEach(func(k, v []byte) error {
						t.Logf("Key: %s, Value length: %d", k, len(v))
						return nil
					})
				}); err != nil {
					t.Errorf("Failed to dump database: %v", err)
				}
			}
		})
	}
}

func TestHistoryLimit(t *testing.T) {
	tmpDB := "test_history.db"
	defer os.Remove(tmpDB)

	histSize := 3
	handler, err := NewEndpointHandler(tmpDB, histSize)
	if err != nil {
		t.Fatalf("Failed to create handler: %v", err)
	}
	defer handler.Close()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer server.Close()

	request := EndpointRequest{
		URL:     server.URL,
		Method:  "GET",
		Timeout: time.Second,
		Status:  http.StatusOK,
	}

	for i := 0; i < histSize+2; i++ {
		handler.Handle(context.Background(), request)
	}

	history, err := handler.GetEndpointHistory(request.URL)
	if err != nil {
		t.Fatalf("Failed to get history: %v", err)
	}

	if len(history) != histSize {
		t.Errorf("History size = %v, want %v", len(history), histSize)
	}
}
