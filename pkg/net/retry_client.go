package net

import (
	"net/http"
	"net/url"
)

// RetryClient is the wrap of http.Client
type RetryClient struct {
	http.Client
	MaxAttempts     int
	currentAttempts int
}

// Do is the wrap of http.Client.Do
func (c *RetryClient) Do(req *http.Request) (rsp *http.Response, err error) {
	rsp, err = c.Client.Do(req)
	if _, ok := err.(*url.Error); ok {
		if c.currentAttempts < c.MaxAttempts {
			c.currentAttempts++
			return c.Do(req)
		}
	}
	return
}
