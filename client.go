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

// OptHTTPMethod sets the HTTP method for the dial
// The default method, if not set, is HTTP CONNECT.
func OptHTTPMethod(method string) clientOption {
	return func(c *client) {
		c.method = method
	}
}

// OptTransport sets a custom http2 transport for the connection
// If set, the custom client will be ignored, and the default http client will be used with the
// custom transport.
func OptTransport(transport *http2.Transport) clientOption {
	return func(c *client) {
		c.transport = transport
	}
}

// OptClient sets a custom http client.
// Make sure to use an HTTP2 transport in this client.
func OptClient(httpClient *http.Client) clientOption {
	return func(c *client) {
		c.httpClient = httpClient
	}
}

// Dial dials an HTTP2 server to establish a full-duplex communication.
//
// Usage:
//      conn, resp, err := h2conn.Dial(ctx, url, h2conn.OptTransport(&http2.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}}))
//      if err != nil {
//          log.Fatalf("Initiate client: %s", err)
//      }
//      if resp.StatusCode != http.StatusOK {
//          log.Fatalf("Bad status code: %d", resp.StatusCode)
//      }
//      defer conn.Close()
//      // use conn
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
