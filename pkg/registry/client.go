package registry

import (
	"net"
	"net/http"
	"time"
)

type Client struct {
	httpClient *http.Client
	Timeout    time.Duration
}

// NewClientWithHTTPClient returns a custom registry client useful for testing
func NewClientWithHTTPClient(httpClient *http.Client) *Client {
	timeout := 30 * time.Second
	return &Client{
		httpClient,
		timeout,
	}
}

// NewClient returns a registry client with the default values
func NewClient() *Client {
	tr := &http.Transport{
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          100,
		IdleConnTimeout:       90 * time.Second,
		TLSHandshakeTimeout:   10 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}
	return NewClientWithHTTPClient(&http.Client{Transport: tr})
}
