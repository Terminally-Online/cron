package endpoint

import (
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/mux"
	"go.etcd.io/bbolt"
)

type EndpointRequest struct {
	URL             string
	Method          string
	Timeout         time.Duration
	Status          int
	RetryAttempts   int
	RetryDelay      time.Duration
	ExpectedContent string
}

type EndpointError struct {
	StatusCode int
	Expected   int
	Message    string
}

type EndpointResponse struct {
	Endpoint  EndpointRequest
	Status    int
	Error     error
	Timestamp time.Time
	Duration  time.Duration
}

type EndpointListResponse struct {
	URLs []string `json:"urls"`
}

type EndpointStats struct {
	TotalChecks      int     `json:"total_checks"`
	SuccessfulChecks int     `json:"successful_checks"`
	UpTimePercentage float64 `json:"uptime_percentage"`
	AverageResponse  int64   `json:"average_response_ms"`
	LastCheck        string  `json:"last_check"`
}

type EndpointResponseStored struct {
	URL       string
	Method    string
	Status    int
	Expected  int
	Error     string
	Timestamp time.Time
	Duration  time.Duration
}

type HistoryResponse struct {
	URL     string         `json:"url"`
	History []HistoryEntry `json:"history"`
	Stats   EndpointStats  `json:"stats"`
}

type HistoryEntry struct {
	Status    int           `json:"status"`
	Expected  int           `json:"expected"`
	Error     string        `json:"error,omitempty"`
	Timestamp time.Time     `json:"timestamp"`
	Duration  time.Duration `json:"duration"`
}

// Configuration types
type RetryConfig struct {
	Attempts int
	Delay    time.Duration
	Timeout  time.Duration
}

// Handler types
type EndpointHandler struct {
	client   *http.Client
	db       *bbolt.DB
	histSize int
}

type Scheduler struct {
	handler   *EndpointHandler
	interval  time.Duration
	endpoints []EndpointRequest
	done      chan struct{}
	wg        sync.WaitGroup
}

type API struct {
	handler *EndpointHandler
	router  *mux.Router
}
