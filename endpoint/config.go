package endpoint

import "time"

var DOMAIN_CONFIG = []DomainRequest{
	{
		Domain: "plug",
		Endpoints: []EndpointRequest{
			{
				URL:             "https://onplug.io",
				Timeout:         5 * time.Second,
				ExpectedContent: "<title>Plug</title>",
			},
			{
				URL:             "https://docs.onplug.io",
				Timeout:         5 * time.Second,
				ExpectedContent: "<title>Introduction | Plug Documentation</title>",
			},
		},
	},
}
