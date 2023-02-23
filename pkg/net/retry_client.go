package net

import (
	"net/http"
	"net/url"
	"strings"
)

// RetryClient is the wrap of http.Client
type RetryClient struct {
	http.Client
	MaxAttempts     int
	currentAttempts int
}

// NewRetryClient creates the instance of RetryClient
func NewRetryClient(client http.Client) *RetryClient {
	return &RetryClient{
		Client:      client,
		MaxAttempts: 3,
	}
}

// Do is the wrap of http.Client.Do
func (c *RetryClient) Do(req *http.Request) (rsp *http.Response, err error) {
	rsp, err = c.Client.Do(req)
	// fmt.Println("client error", err, c.Client.Timeout, reflect.TypeOf(err))

	if _, ok := err.(*url.Error); ok && !strings.Contains(err.Error(), "context canceled") {
		// fmt.Println("retry", c.currentAttempts, c.MaxAttempts)
		if c.currentAttempts < c.MaxAttempts {
			c.currentAttempts++
			// fmt.Println("try", c.currentAttempts)
			return c.Do(req)
		}
	}
	return
}
