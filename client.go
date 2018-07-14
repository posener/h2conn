package h2conn

import (
	"context"
	"io"
	"net/http"

	"golang.org/x/net/http2"
)

var DefaultClient = &http.Client{
	Transport: &http2.Transport{},
}

type client struct {
	method     string
	transport  *http2.Transport
	httpClient *http.Client
}

type clientOption func(*client)

func OptHTTPMethod(method string) clientOption {
	return func(c *client) {
		c.method = method
	}
}

func OptTransport(transport *http2.Transport) clientOption {
	return func(c *client) {
		c.transport = transport
	}
}

func OptClient(httpClient *http.Client) clientOption {
	return func(c *client) {
		c.httpClient = httpClient
	}
}

func Dial(ctx context.Context, url string, opt ...clientOption) (*Conn, *http.Response, error) {
	var cl = client{
		method:     http.MethodConnect,
		httpClient: DefaultClient,
	}
	for _, mod := range opt {
		mod(&cl)
	}

	if cl.transport != nil {
		cl.httpClient = &http.Client{Transport: cl.transport}
	}

	pr, pw := io.Pipe()
	req, err := http.NewRequest(cl.method, url, pr)
	if err != nil {
		return nil, nil, err
	}

	// apply given context to the sent request
	req = req.WithContext(ctx)

	resp, err := cl.httpClient.Do(req)
	if err != nil {
		return nil, nil, err
	}

	return newConn(req.Context(), resp.Body, pw), resp, nil
}
