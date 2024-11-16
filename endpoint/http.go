package endpoint

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/gorilla/mux"
)

type API struct {
	handler *EndpointHandler
	router  *mux.Router
}

type EndpointListResponse struct {
	URLs []string `json:"urls"`
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

type EndpointStats struct {
	TotalChecks      int     `json:"total_checks"`
	SuccessfulChecks int     `json:"successful_checks"`
	UpTimePercentage float64 `json:"uptime_percentage"`
	AverageResponse  int64   `json:"average_response_ms"`
	LastCheck        string  `json:"last_check"`
}

func NewAPI(handler *EndpointHandler) *API {
	api := &API{
		handler: handler,
		router:  mux.NewRouter(),
	}
	api.setupRoutes()
	return api
}

func (a *API) setupRoutes() {
	a.router.HandleFunc("/endpoints", a.handleGetEndpoints).Methods("GET")
	a.router.HandleFunc("/endpoint/history", a.handleGetEndpointHistory).Methods("GET")
}

func (a *API) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	a.router.ServeHTTP(w, r)
}

func (a *API) handleGetEndpoints(w http.ResponseWriter, r *http.Request) {
	endpoints, err := a.handler.GetAllEndpoints()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	response := EndpointListResponse{
		URLs: endpoints,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
}

func (a *API) handleGetEndpointHistory(w http.ResponseWriter, r *http.Request) {
	url := r.URL.Query().Get("url")
	if url == "" {
		http.Error(w, "URL parameter is required", http.StatusBadRequest)
		return
	}

	history, err := a.handler.GetEndpointHistory(url)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if len(history) == 0 {
		http.Error(w, "Endpoint not found", http.StatusNotFound)
		return
	}

	historyEntries := make([]HistoryEntry, len(history))
	var successfulChecks int
	var totalDuration time.Duration

	for i, entry := range history {
		var errorStr string
		if entry.Error != nil {
			errorStr = entry.Error.Error()
		}

		historyEntries[i] = HistoryEntry{
			Status:    entry.Status,
			Expected:  entry.Endpoint.Status,
			Error:     errorStr,
			Timestamp: entry.Timestamp,
			Duration:  entry.Duration,
		}

		if entry.Error == nil {
			successfulChecks++
		}
		totalDuration += entry.Duration
	}

	stats := EndpointStats{
		TotalChecks:      len(history),
		SuccessfulChecks: successfulChecks,
		UpTimePercentage: float64(successfulChecks) / float64(len(history)) * 100,
		AverageResponse:  totalDuration.Milliseconds() / int64(len(history)),
		LastCheck:        history[len(history)-1].Timestamp.Format(time.RFC3339),
	}

	response := HistoryResponse{
		URL:     url,
		History: historyEntries,
		Stats:   stats,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
}

