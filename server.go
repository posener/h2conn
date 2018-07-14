package h2conn

import (
	"fmt"
	"io"
	"net/http"
)

var ErrHttp2NotSupported = fmt.Errorf("HTTP2 not supported")

type Upgrader struct {
	StatusCode int
}

var defaultUpgrader = Upgrader{
	StatusCode: http.StatusOK,
}

func Upgrade(w http.ResponseWriter, r *http.Request) (*Conn, error) {
	return defaultUpgrader.Upgrade(w, r)
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
func (u *Upgrader) Upgrade(w http.ResponseWriter, r *http.Request) (*Conn, error) {

	flusher, ok := w.(http.Flusher)
	if !ok {
		return nil, ErrHttp2NotSupported
	}

	//clientClosed, ok := w.(http.CloseNotifier)
	//if !ok {
	//	return nil, ErrHttp2NotSupported
	//}

	c := newConn(r.Context(), r.Body, &writer{w: w, f: flusher}, r.Host, r.RemoteAddr)

	w.WriteHeader(u.StatusCode)
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
