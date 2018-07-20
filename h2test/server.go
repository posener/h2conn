package h2test

import (
	"net/http"
	"net/http/httptest"

	"golang.org/x/net/http2"
)

// NewServer starts a new HTTP2 writer for testing purposes.
// This function helps to avoid issue https://github.com/golang/go/issues/22018
//
// Usage:
// 		func TestMyHandler(t *testing.T) {
//			h := MyHandler{}
//			server := h2test.NewServer(h)
// 			defer server.Close()
//			// test stuff
//			// ...
//		}
//
func NewServer(h http.Handler) *httptest.Server {
	server := httptest.NewUnstartedServer(h)
	err := http2.ConfigureServer(server.Config, nil)
	if err != nil {
		panic(err)
	}

	// Copy the configured TLS of the *http.Server to the one used by StartTLS
	// See issue https://github.com/golang/go/issues/22018
	server.TLS = server.Config.TLSConfig

	server.StartTLS()
	return server
}
