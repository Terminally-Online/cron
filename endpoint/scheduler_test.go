package endpoint

import (
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"
)

func TestScheduler(t *testing.T) {
	tmpDB := "test_scheduler.db"
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
		}
	}))
	defer server.Close()

	endpoints := []EndpointRequest{
		{
			URL:           server.URL + "/success",
			Method:        "GET",
			Timeout:       time.Second,
			Status:        http.StatusOK,
			RetryAttempts: 2,
			RetryDelay:    100 * time.Millisecond,
		},
		{
			URL:           server.URL + "/error",
			Method:        "GET",
			Timeout:       time.Second,
			Status:        http.StatusInternalServerError,
			RetryAttempts: 2,
			RetryDelay:    100 * time.Millisecond,
		},
	}

	t.Run("scheduler lifecycle", func(t *testing.T) {
		scheduler := NewScheduler(handler, 200*time.Millisecond, endpoints)

		scheduler.Start()

		checkCycles := 2
		waitTime := scheduler.interval*time.Duration(checkCycles) + 100*time.Millisecond
		time.Sleep(waitTime)

		scheduler.Stop()

		time.Sleep(200 * time.Millisecond)

		for _, ep := range endpoints {
			history, err := handler.GetEndpointHistory(ep.URL)
			if err != nil {
				t.Errorf("Failed to get history for %s: %v", ep.URL, err)
				continue
			}

			if len(history) == 0 {
				t.Errorf("No history entries found for %s", ep.URL)
				continue
			}
		}
	})

	t.Run("concurrent checks", func(t *testing.T) {
		scheduler := NewScheduler(handler, time.Minute, endpoints)
		start := time.Now()
		scheduler.checkAll()
		duration := time.Since(start)

		maxExpectedDuration := time.Second * 3
		if duration > maxExpectedDuration {
			t.Errorf("Concurrent checks took too long: %v > %v", duration, maxExpectedDuration)
		}
	})
}
