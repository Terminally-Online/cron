package endpoint

import (
	"context"
	"log"
	"sync"
	"time"
)

func NewScheduler(handler *EndpointHandler, interval time.Duration, endpoints []EndpointRequest) *Scheduler {
	return &Scheduler{
		handler:   handler,
		interval:  interval,
		endpoints: endpoints,
		done:      make(chan struct{}),
	}
}

func (s *Scheduler) Start() {
	s.wg.Add(1)
	go s.run()
	log.Printf("Scheduler started with %d endpoints at %v interval", len(s.endpoints), s.interval)
}

func (s *Scheduler) Stop() {
	close(s.done)
	s.wg.Wait()
}

func (s *Scheduler) run() {
	defer s.wg.Done()

	s.checkAll()

	ticker := time.NewTicker(s.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			s.checkAll()
		case <-s.done:
			return
		}
	}
}

func (s *Scheduler) checkAll() {
	var wg sync.WaitGroup
	results := make(chan EndpointResponse, len(s.endpoints))

	for _, ep := range s.endpoints {
		wg.Add(1)
		go func(endpoint EndpointRequest) {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), endpoint.Timeout)
			defer cancel()

			result := s.handler.Handle(ctx, endpoint)
			results <- result
		}(ep)
	}

	go func() {
		wg.Wait()
		close(results)
	}()
}
