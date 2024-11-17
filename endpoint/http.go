package endpoint

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/gorilla/mux"
)

func NewAPI(handler *EndpointHandler) *API {
	api := &API{
		handler: handler,
		router:  mux.NewRouter(),
	}
	api.setupRoutes()
	return api
}

func (a *API) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	a.router.ServeHTTP(w, r)
}

func (a *API) setupRoutes() {
	a.router.HandleFunc("/endpoints", a.handleGetEndpoints).Methods("GET")
	a.router.HandleFunc("/endpoint/history", a.handleGetEndpointHistory).Methods("GET")
	a.router.HandleFunc("/domain/history", a.handleGetDomainHistory).Methods("GET")
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

	for _, entry := range history {
		var errorStr string
		if entry.Error != nil {
			errorStr = entry.Error.Error()
		}

		historyEntries = append(historyEntries, HistoryEntry{
			Status:    entry.Status,
			Expected:  entry.Endpoint.Status,
			Error:     errorStr,
			Timestamp: entry.Timestamp,
			Duration:  entry.Duration,
		})

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

func (a *API) handleGetDomainHistory(w http.ResponseWriter, r *http.Request) {
	domain := r.URL.Query().Get("domain")
	if domain == "" {
		http.Error(w, "Domain parameter is required", http.StatusBadRequest)
		return
	}

	endpoints, err := a.handler.GetDomainEndpoints(domain)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	if len(endpoints) == 0 {
		http.Error(w, "Domain not found", http.StatusNotFound)
		return
	}

	endpointResponses := make([]HistoryResponse, 0, len(endpoints))
	for _, endpoint := range endpoints {
		history, err := a.handler.GetEndpointHistory(endpoint)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		if len(history) > 0 {
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

			endpointResponses = append(endpointResponses, HistoryResponse{
				URL:     endpoint,
				History: historyEntries,
				Stats:   stats,
			})
		}
	}

	response := DomainResponse{
		Domain:    domain,
		Endpoints: endpointResponses,
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		http.Error(w, "Failed to encode response", http.StatusInternalServerError)
		return
	}
}
