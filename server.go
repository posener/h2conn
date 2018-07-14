package h2conn

import (
	"fmt"
	"io"
	"net/http"
)

type server struct {
	statusCode int
}

type serverOption func(*server)

func OptStatusCode(statusCode int) serverOption {
	return func(s *server) {
		s.statusCode = statusCode
	}
}

func New(w http.ResponseWriter, r *http.Request, opt ...serverOption) (*Conn, error) {
	srv := server{
		statusCode: http.StatusOK,
	}
	for _, mod := range opt {
		mod(&srv)
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		return nil, fmt.Errorf("not an http2 connection")
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
	return nil
}
