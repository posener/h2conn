package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/posener/h2conn"
	"github.com/posener/h2conn/h2test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/http2"
)

func TestHandler(t *testing.T) {
	t.Parallel()

	server := h2test.NewServer(handler{})
	defer server.Close()

	// We use a client with custom http2.Transport since the server certificate is not signed by
	// an authorized CA, and this is the way to ignore certificate verification errors.
	d := &h2conn.Dialer{
		Client: &http.Client{
			Transport: &http2.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}},
		},
	}
	conn, resp, err := d.Dial(context.Background(), server.URL, nil)
	require.NoError(t, err)

	defer conn.Close()

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	in, out := json.NewDecoder(conn), json.NewEncoder(conn)

	err = out.Encode("hello")
	require.NoError(t, err)

	var msg string
	err = in.Decode(&msg)
	require.NoError(t, err)

	assert.Equal(t, "hello", msg)
}
