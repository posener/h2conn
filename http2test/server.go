package http2test

import (
	"net/http"
	"net/http/httptest"

	"golang.org/x/net/http2"
)

func NewServer(h http.Handler) *httptest.Server {
	server := httptest.NewUnstartedServer(h)
	err := http2.ConfigureServer(server.Config, nil)
	if err != nil {
		panic(err)
	}

	// Copy the configured TLS of the *http.Server to the one used by StartTLS
	server.TLS = server.Config.TLSConfig

	server.StartTLS()
	return server
}
