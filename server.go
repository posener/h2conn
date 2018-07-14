package h2conn

import (
	"fmt"
	"io"
	"net/http"
)

var ErrHttp2NotSupported = fmt.Errorf("HTTP2 not supported")

type server struct {
	statusCode int
}

type serverOption func(*server)

// OptStatusCode sets the status code that the server returns to the client if upgrade was successful.
func OptStatusCode(statusCode int) serverOption {
	return func(s *server) {
		s.statusCode = statusCode
	}
}

// Upgrade is used on a server http.Handler.
// It handles a request and "upgrade" the request connection to a websocket-like
// full-duplex communication.
// If the client does not support HTTP2, an ErrHttp2NotSupported is returned.
//
// Usage:
//
//      func (w http.ResponseWriter, r *http.Request) {
//          conn, err := h2conn.Upgrade(w, r)
//          if err != nil {
//		        log.Printf("Failed creating http2 connection: %s", err)
//		        http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
//		        return
//	        }
//          // use conn
//      }
func Upgrade(w http.ResponseWriter, r *http.Request, opt ...serverOption) (*Conn, error) {
	srv := server{
		statusCode: http.StatusOK,
	}
	for _, mod := range opt {
		mod(&srv)
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		return nil, ErrHttp2NotSupported
	}

	c := newConn(r.Context(), r.Body, &writer{w: w, f: flusher})

	w.WriteHeader(srv.statusCode)
	flusher.Flush()

	return c, nil
}

type writer struct {
	w io.Writer
	f http.Flusher
}

func (w *writer) Write(data []byte) (int, error) {
	n, err := w.w.Write(data)
	w.f.Flush()
	return n, err
}

func (w *writer) Close() error {
	// TODO: implement close
	return nil
}
