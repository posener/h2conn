package h2conn

import (
	"fmt"
	"io"
	"net/http"
)

func New(w http.ResponseWriter, r *http.Request) (*Conn, error) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		return nil, fmt.Errorf("not an http2 connection")
	}

	c := newConn(r.Context(), r.Body, &writer{w: w, f: flusher})

	// TODO: make status code this configurable
	w.WriteHeader(http.StatusCreated)
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
