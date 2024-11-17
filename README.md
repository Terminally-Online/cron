# Cron Service

A lightweight and robust health checking service written in Go that monitors endpoint availability and provides historical uptime data.

## Features

-   Concurrent endpoint health monitoring
-   Configurable check intervals
-   Persistent storage of check results using BoltDB
-   RESTful API for accessing health check data
-   Historical uptime statistics
-   Graceful shutdown handling
-   Support for custom HTTP methods and expected status codes

## Installation

```bash
# Clone the repository
git clone https://github.com/Terminally-Online/cron

# Install dependencies
go mod download
```

## Configuration

Configure your endpoints in `endpoint/config.go`:

```go
var ENDPOINT_CONFIG = []EndpointRequest{
    {
        URL: "https://onplug.io",
        Timeout: 5 * time.Second,
    },
    {
        URL: "https://docs.onplug.io",
        Timeout: 5 * time.Second,
    },
}
```

## Usage

### Running the Service

To run the process in the background (non-blocking) execute the already built package:

```bash
./cron &
```

### API Endpoints

#### List All Monitored Endpoints

```http
GET /endpoints
```

Response:

```json
{
    "urls": ["https://onplug.io", "https://docs.onplug.io"]
}
```

#### Get Endpoint History

```http
GET /endpoint/history?url=https://onplug.io
```

Response:

```json
{
    "url": "https://onplug.io",
    "history": [
        {
            "status": 200,
            "expected": 200,
            "timestamp": "2024-11-15T10:00:00Z",
            "duration": 123000000,
            "error": null
        }
    ],
    "stats": {
        "total_checks": 48,
        "successful_checks": 47,
        "uptime_percentage": 97.92,
        "average_response_ms": 156,
        "last_check": "2024-11-15T10:00:00Z"
    }
}
```

## TODO

-   [ ] Add metrics collection (Prometheus)
-   [ ] Implement alerting for consecutive failures
-   [ ] Support for different intervals per endpoint
-   [ ] Add webhook notifications
