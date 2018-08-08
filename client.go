package h2conn

import (
	"context"
	"io"
	"net/http"

	"golang.org/x/net/http2"
)

// Client provides HTTP2 client side connection with special arguments
type Client struct {
	// Method sets the HTTP method for the dial
	// The default method, if not set, is HTTP POST.
	Method string
	// Header enables sending custom headers to the server
	Header http.Header
	// Client is a custom HTTP client to be used for the connection.
	// The client must have an http2.Transport as it's transport.
	Client *http.Client
}

// Connect establishes a full duplex communication with an HTTP2 server with custom client.
// See h2conn.Connect documentation for more info.
func (c *Client) Connect(ctx context.Context, urlStr string) (*Conn, *http.Response, error) {
	reader, writer := io.Pipe()

	// Create a request object to send to the server
	req, err := http.NewRequest(c.Method, urlStr, reader)
	if err != nil {
		return nil, nil, err
	}

	// Apply custom headers
	if c.Header != nil {
		req.Header = c.Header
	}

	// Apply given context to the sent request
	req = req.WithContext(ctx)

	// If an http client was not defined, use the default http client
	httpClient := c.Client
	if httpClient == nil {
		httpClient = defaultClient.Client
	}

	// Perform the request
	resp, err := httpClient.Do(req)
	if err != nil {
		return nil, nil, err
	}

	// Create a connection
	conn, ctx := newConn(req.Context(), resp.Body, writer)

	// Apply the connection context on the request context
	resp.Request = req.WithContext(ctx)

	return conn, resp, nil
}

var defaultClient = Client{
	Method: http.MethodPost,
	Client: &http.Client{Transport: &http2.Transport{}},
}

// Connect establishes a full duplex communication with an HTTP2 server.
//
// Usage:
//
//      conn, resp, err := h2conn.Connect(ctx, url)
//      if err != nil {
//          log.Fatalf("Initiate client: %s", err)
//      }
//      if resp.StatusCode != http.StatusOK {
//          log.Fatalf("Bad status code: %d", resp.StatusCode)
//      }
//      defer conn.Close()
//
//      // use conn
//
func Connect(ctx context.Context, urlStr string) (*Conn, *http.Response, error) {
	return defaultClient.Connect(ctx, urlStr)
}
