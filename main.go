package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"terminally-online/cron/endpoint"
	"time"
)

func main() {
    handler, err := endpoint.NewEndpointHandler("endpoints.db", 48)
    if err != nil {
        log.Fatalf("Failed to create endpoint handler: %v", err)
    }
    defer handler.Close()

    api := endpoint.NewAPI(handler)

    var endpoints []endpoint.EndpointRequest
    for _, domain := range endpoint.DOMAIN_CONFIG {
        endpoints = append(endpoints, domain.Endpoints...)
    }

    scheduler := endpoint.NewScheduler(
        handler,
        1*time.Hour,
        endpoints,
    )

    scheduler.Start()

    srv := &http.Server{
        Handler:      api,
        Addr:         ":8080",
        WriteTimeout: 15 * time.Second,
        ReadTimeout:  15 * time.Second,
    }

    stop := make(chan os.Signal, 1)
    signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

    go func() {
        log.Printf("Starting server on :8080")
        if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
            log.Fatalf("Error starting server: %v", err)
        }
    }()

    <-stop
    log.Println("Shutting down...")

    scheduler.Stop()

    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    if err := srv.Shutdown(ctx); err != nil {
        log.Printf("Error shutting down server: %v", err)
    }

    log.Println("Shutdown complete")
}
