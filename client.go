package h2conn

import (
	"context"
	"io"
	"net/http"

	"golang.org/x/net/http2"
)

type Dialer struct {
	// Method sets the HTTP method for the dial
	// The default method, if not set, is HTTP CONNECT.
	Method string
	Client *http.Client
}

var defaultDialer = Dialer{
	Method: http.MethodConnect,
	Client: &http.Client{Transport: &http2.Transport{}},
}

func Dial(ctx context.Context, urlStr string, header http.Header) (*Conn, *http.Response, error) {
	return defaultDialer.Dial(ctx, urlStr, header)
}

// Dial dials an HTTP2 server to establish a full-duplex communication.
// Similar API to the net.DialContext function.
//
// Usage:
//
//      conn, resp, err := h2conn.Dial(ctx, url)
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
func (d *Dialer) Dial(ctx context.Context, urlStr string, header http.Header) (*Conn, *http.Response, error) {
	pr, pw := io.Pipe()
	req, err := http.NewRequest(d.Method, urlStr, pr)
	if err != nil {
		return nil, nil, err
	}

	if header != nil {
		req.Header = header
	}

	// apply given context to the sent request
	req = req.WithContext(ctx)

	resp, err := d.Client.Do(req)
	if err != nil {
		return nil, nil, err
	}

	return newConn(req.Context(), resp.Body, pw, resp.Request.RemoteAddr, req.Host), resp, nil
}
