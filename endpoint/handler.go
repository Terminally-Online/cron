package endpoint

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"go.etcd.io/bbolt"
)

type StoredResponse struct {
	URL       string
	Method    string
	Status    int
	Expected  int
	Error     string
	Timestamp time.Time
	Duration  time.Duration
}

type EndpointRequest struct {
	URL     string
	Method  string
	Timeout time.Duration
	Status  int
}

type EndpointResponse struct {
	Endpoint  EndpointRequest
	Status    int
	Error     error
	Timestamp time.Time
	Duration  time.Duration
}

type EndpointHandler struct {
	client   *http.Client
	db       *bbolt.DB
	histSize int
}

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
	response := h.performRequest(ctx, endpointRequest)
	if err := h.storeResponse(response); err != nil {

		fmt.Printf("failed to store response: %v\n", err)
	}
	return response
}

func (h *EndpointHandler) performRequest(ctx context.Context, endpointRequest EndpointRequest) EndpointResponse {
	start := time.Now()
	endpointResponse := EndpointResponse{
		Endpoint:  endpointRequest,
		Timestamp: start,
	}

	if endpointRequest.Timeout == 0 {
		endpointRequest.Timeout = 10 * time.Second
	}
	if endpointRequest.Status == 0 {
		endpointRequest.Status = http.StatusOK
	}
	if endpointRequest.Method != "GET" && endpointRequest.Method != "POST" {
		endpointRequest.Method = "GET"
	}

	ctx, cancel := context.WithTimeout(ctx, endpointRequest.Timeout)
	defer cancel()

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
	if response.StatusCode != endpointRequest.Status {
		endpointResponse.Error = fmt.Errorf("unexpected status code: got %d, wanted %d",
			response.StatusCode, endpointRequest.Status)
	}

	return endpointResponse
}

func (h *EndpointHandler) storeResponse(response EndpointResponse) error {
	stored := StoredResponse{
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

		var responses []StoredResponse
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

func (h *EndpointHandler) GetEndpointHistory(url string) ([]EndpointResponse, error) {
	var responses []EndpointResponse

	err := h.db.View(func(tx *bbolt.Tx) error {
		b := tx.Bucket([]byte(endpointBucket))
		data := b.Get([]byte(url))
		if data == nil {
			return nil
		}

		var stored []StoredResponse
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

