package http

import "net/http"

type ClientInterface interface {
	Get(url string) (*http.Response, error)
}

type Client struct {
	http.Client
}

func (c *Client) Get(url string) (*http.Response, error) {
	return c.Client.Get(url)
}
