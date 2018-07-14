package h2conn

import (
	"context"
	"crypto/tls"
	"io"
	"net/http"

	"golang.org/x/net/http2"
)

func Dial(ctx context.Context, url string, tls *tls.Config) (*Conn, *http.Response, error) {
	pr, pw := io.Pipe()
	// TODO: make method configurable
	req, err := http.NewRequest(http.MethodConnect, url, pr)
	if err != nil {
		return nil, nil, err
	}

	// apply given context to the sent request
	req = req.WithContext(ctx)

	// TODO: add option for custom client
	client := &http.Client{
		Transport: &http2.Transport{
			TLSClientConfig: tls,
		},
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, nil, err
	}

	return newConn(req.Context(), resp.Body, pw), resp, nil
}
