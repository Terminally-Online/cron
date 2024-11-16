package endpoint

import "time"

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
