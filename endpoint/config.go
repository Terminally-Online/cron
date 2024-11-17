package endpoint

import "time"

var ENDPOINT_CONFIG = []EndpointRequest{
	{
		URL:           "https://onplug.io",
		Timeout:       5 * time.Second,
		RetryAttempts: 3,
		RetryDelay:    1 * time.Second,
		ExpectedContent: "<title>Plug</title>",
	},
	{
		URL:           "https://docs.onplug.io",
		Timeout:       5 * time.Second,
		RetryAttempts: 3,
		RetryDelay:    1 * time.Second,
	},
}
