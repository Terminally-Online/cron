package endpoint

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"terminally-online/cron/utils"
	"time"

	"go.etcd.io/bbolt"
)

const endpointBucket = "endpoints"

func NewEndpointHandler(dbPath string, histSize int) (*EndpointHandler, error) {
	if histSize <= 0 {
		histSize = 48
	}

	db, err := bbolt.Open(dbPath, 0600, &bbolt.Options{
		Timeout: 1 * time.Second,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	err = db.Update(func(tx *bbolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists([]byte(endpointBucket))
		return err
	})
	if err != nil {
		db.Close()
		return nil, fmt.Errorf("failed to create bucket: %w", err)
	}

	return &EndpointHandler{
		client:   &http.Client{},
		db:       db,
		histSize: histSize,
	}, nil
}

func (h *EndpointHandler) Close() error {
	return h.db.Close()
}

func (h *EndpointHandler) Handle(ctx context.Context, endpointRequest EndpointRequest) EndpointResponse {
	retryConfig := h.getRetryConfig(endpointRequest)
	timeoutCtx, cancel := context.WithTimeout(ctx, retryConfig.Timeout)
	defer cancel()

	var response EndpointResponse
	var lastError error

	for attempt := 0; attempt <= retryConfig.Attempts; attempt++ {
		if attempt > 0 {
			if err := h.waitForRetry(timeoutCtx, attempt, retryConfig); err != nil {
				return h.createTimeoutResponse(endpointRequest, attempt, lastError)
			}
		}

		response = h.performRequest(timeoutCtx, endpointRequest)
		if h.isSuccessfulResponse(response) {
			break
		}
		lastError = response.Error
	}

	if err := h.storeResponse(response); err != nil {
		log.Printf("Failed to store response: %v", err)
	}

	return response
}

func (h *EndpointHandler) GetEndpointHistory(url string) ([]EndpointResponse, error) {
	var responses []EndpointResponse

	err := h.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(endpointBucket))
		data := b.Get([]byte(url))
		if data == nil {
			return nil
		}

		var stored []EndpointResponseStored
		if err := json.Unmarshal(data, &stored); err != nil {
			return fmt.Errorf("failed to unmarshal responses: %w", err)
		}

		responses = make([]EndpointResponse, len(stored))
		for i, s := range stored {
			responses[i] = EndpointResponse{
				Endpoint: EndpointRequest{
					URL:    s.URL,
					Method: s.Method,
					Status: s.Expected,
				},
				Status:    s.Status,
				Timestamp: s.Timestamp,
				Duration:  s.Duration,
			}
			if s.Error != "" {
				responses[i].Error = fmt.Errorf("%s", s.Error)
			}
		}

		return nil
	})

	return responses, err
}

func (h *EndpointHandler) GetAllEndpoints() ([]string, error) {
	var urls []string

	err := h.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(endpointBucket))
		return b.ForEach(func(k, v []byte) error {
			urls = append(urls, string(k))
			return nil
		})
	})

	return urls, err
}

func (h *EndpointHandler) isSuccessfulResponse(resp EndpointResponse) bool {
	if resp.Status >= 400 {
		return false
	}
	return resp.Status == resp.Endpoint.Status
}

func (h *EndpointHandler) createTimeoutResponse(req EndpointRequest, attempt int, lastError error) EndpointResponse {
	response := EndpointResponse{
		Endpoint:  req,
		Status:    0,
		Error:     fmt.Errorf("timeout reached after %d retries: %w", attempt, lastError),
		Timestamp: time.Now(),
	}

	if err := h.storeResponse(response); err != nil {
		log.Printf("Failed to store timeout response: %v", err)
	}

	return response
}

func (h *EndpointHandler) getRetryConfig(req EndpointRequest) RetryConfig {
	return RetryConfig{
		Attempts: utils.DefaultIfZero(req.RetryAttempts, 3),
		Delay:    utils.DefaultIfZero(req.RetryDelay, time.Second),
		Timeout:  utils.DefaultIfZero(req.Timeout, 10*time.Second),
	}
}

func (h *EndpointHandler) waitForRetry(ctx context.Context, attempt int, config RetryConfig) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(config.Delay):
		log.Printf("Retry attempt %d/%d", attempt, config.Attempts)
		return nil
	}
}

func (h *EndpointHandler) performRequest(ctx context.Context, endpointRequest EndpointRequest) EndpointResponse {
	start := time.Now()
	endpointResponse := EndpointResponse{
		Endpoint:  endpointRequest,
		Timestamp: start,
	}

	request, err := http.NewRequestWithContext(ctx, endpointRequest.Method, endpointRequest.URL, nil)
	if err != nil {
		endpointResponse.Error = fmt.Errorf("failed to create request: %w", err)
		return endpointResponse
	}

	response, err := h.client.Do(request)
	endpointResponse.Duration = time.Since(start)

	if err != nil {
		endpointResponse.Error = fmt.Errorf("request failed: %w", err)
		return endpointResponse
	}
	defer response.Body.Close()

	endpointResponse.Status = response.StatusCode

	// Always set error for non-2xx status codes, even if expected
	if response.StatusCode >= 400 {
		endpointResponse.Error = fmt.Errorf("received error status code: %d", response.StatusCode)
	} else if response.StatusCode != endpointRequest.Status {
		endpointResponse.Error = fmt.Errorf("unexpected status code: got %d, wanted %d",
			response.StatusCode, endpointRequest.Status)
	}

	return endpointResponse
}

func (h *EndpointHandler) storeResponse(response EndpointResponse) error {
	stored := EndpointResponseStored{
		URL:       response.Endpoint.URL,
		Method:    response.Endpoint.Method,
		Status:    response.Status,
		Expected:  response.Endpoint.Status,
		Timestamp: response.Timestamp,
		Duration:  response.Duration,
	}
	if response.Error != nil {
		stored.Error = response.Error.Error()
	}

	return h.db.Update(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(endpointBucket))

		var responses []EndpointResponseStored
		data := b.Get([]byte(response.Endpoint.URL))
		if data != nil {
			if err := json.Unmarshal(data, &responses); err != nil {
				return fmt.Errorf("failed to unmarshal existing responses: %w", err)
			}
		}

		responses = append(responses, stored)
		if len(responses) > h.histSize {
			responses = responses[len(responses)-h.histSize:]
		}

		data, err := json.Marshal(responses)
		if err != nil {
			return fmt.Errorf("failed to marshal responses: %w", err)
		}

		return b.Put([]byte(response.Endpoint.URL), data)
	})
}
