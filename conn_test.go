package h2conn

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"encoding/gob"
	"encoding/json"
	"io"
	"log"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"fmt"

	"encoding/binary"

	"github.com/posener/h2conn/h2test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/http2"
)

const (
	numRequests   = 100
	shortDuration = 300 * time.Millisecond
)

var insecureClient = Client{
	Client: &http.Client{
		Transport: &http2.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}},
	},
}

// TestConcurrent runs a simple test of concurrent reads and writes on an HTTP2 full duplex connection
func TestConcurrent(t *testing.T) {
	t.Parallel()

	var serverConn *Conn

	server := h2test.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var err error
		serverConn, err = Accept(w, r)
		if err != nil {
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
		defer serverConn.Close()

		// simple read loop that echos the upper case of what was read.
		buf := bufio.NewReader(serverConn)
		for {
			msg, _, err := buf.ReadLine()
			if err != nil {
				log.Printf("Server failed read: %s", err)
				break
			}

			_, err = serverConn.Write(append(bytes.ToUpper(msg), '\n'))
			if err != nil {
				log.Printf("Server failed write: %s", err)
				break
			}
		}
	}))
	defer server.Close()

	clientConn, resp, err := insecureClient.Connect(context.Background(), server.URL)
	require.Nil(t, err)

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	buf := bufio.NewReader(clientConn)

	var wg sync.WaitGroup
	wg.Add(numRequests)

	go func() {
		for i := 0; i < numRequests; i++ {
			_, err := clientConn.Write([]byte("hello\n"))
			require.Nil(t, err)
		}
	}()

	go func() {
		for i := 0; i < numRequests; i++ {
			msg, _, err := buf.ReadLine()
			require.Nil(t, err)
			assert.Equal(t, "HELLO", string(msg))
			wg.Done()
		}
	}()
	wg.Wait()

	clientConn.Close()
}

// TestClientClose tests that server gets io.EOF after client closed the connection
func TestClientClose(t *testing.T) {
	t.Parallel()

	server, serverAccepted, serverHandlerWait := startServer()
	defer server.Close()
	defer close(serverHandlerWait)

	clientConn, resp, err := insecureClient.Connect(context.Background(), server.URL)
	require.Nil(t, err)

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	serverConn := <-serverAccepted

	// close client connection
	clientConn.Close()

	// test that read from server returns an io.EOF error
	var buf = make([]byte, 100)
	n, err := serverConn.Read(buf)
	assert.Equal(t, io.EOF, err)
	assert.Equal(t, 0, n)
}

// TestServer tests that client gets io.EOF after server closed the connection
func TestServerClose(t *testing.T) {
	t.Parallel()

	server, serverAccepted, serverHandlerWait := startServer()
	defer server.Close()

	clientConn, resp, err := insecureClient.Connect(context.Background(), server.URL)
	require.Nil(t, err)

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	serverConn := <-serverAccepted

	// close server connection
	serverConn.Close()
	close(serverHandlerWait)

	// test that read from server returns an io.EOF error
	var buf = make([]byte, 100)
	n, err := clientConn.Read(buf)
	assert.Equal(t, io.EOF, err)
	assert.Equal(t, 0, n)
}

func TestSpecialCases(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		server  func(*testing.T) *httptest.Server
		client  func() *Client
		wantErr bool
	}{
		{
			name:   "insecure transport",
			server: nopHandler,
			client: func() *Client {
				return &Client{Client: &http.Client{Transport: &http2.Transport{}}}
			},
			wantErr: true,
		},
		{
			name:   "invalid request",
			server: nopHandler,
			client: func() *Client {
				cl := insecureClient
				cl.Method = "\n"
				return &cl
			},
			wantErr: true,
		},
		{
			name: "headers",
			client: func() *Client {
				cl := insecureClient
				cl.Header = http.Header{"Foo": []string{"bar"}}
				return &cl
			},
			server: func(*testing.T) *httptest.Server {
				return h2test.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					_, err := Accept(w, r)
					require.NoError(t, err)
					assert.Equal(t, "bar", r.Header.Get("Foo"))
				}))
			},
		},
		{
			name: "server use http1",
			server: func(*testing.T) *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					_, err := Accept(w, r)
					assert.Error(t, err)
				}))
			},
			wantErr: true,
		},
		{
			name: "server and client use http1",
			client: func() *Client {
				return &Client{Client: &http.Client{
					// Timeout must be set, otherwise the client hangs because of an open client writer.
					Timeout: shortDuration,
				}}
			},
			server: func(*testing.T) *httptest.Server {
				return httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					_, err := Accept(w, r)
					assert.Error(t, err)
				}))
			},
			wantErr: true,
		},
		{
			name: "client use http1 transport",
			client: func() *Client {
				return &Client{Client: &http.Client{
					Transport: &http.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}},
					// Timeout must be set, otherwise the client hangs because of an open client writer.
					Timeout: shortDuration,
				}}
			},
			server: func(*testing.T) *httptest.Server {
				return h2test.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					_, err := Accept(w, r)
					assert.Error(t, err)
				}))
			},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			server := tt.server(t)
			defer server.Close()

			cl := &insecureClient
			if tt.client != nil {
				cl = tt.client()
			}

			conn, resp, err := cl.Connect(context.Background(), server.URL)
			if tt.wantErr {
				assert.Error(t, err)
				return
			}
			require.NoError(t, err)
			assert.Equal(t, http.StatusOK, resp.StatusCode)
			conn.Close()
		})
	}
}

// TestFormat tests sending JSON and GOB formats over h2conn
func TestFormat(t *testing.T) {
	t.Parallel()

	server, serverAccepted, serverHandlerWait := startServer()
	defer server.Close()
	defer close(serverHandlerWait)

	clientConn, resp, err := insecureClient.Connect(context.Background(), server.URL)
	require.Nil(t, err)

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	serverConn := <-serverAccepted

	serverJSONIn, serverJSONOut := json.NewDecoder(serverConn), json.NewEncoder(serverConn)
	clientJSONIn, clientJSONOut := json.NewDecoder(clientConn), json.NewEncoder(clientConn)

	serverGOBIn, serverGOBOut := gob.NewDecoder(serverConn), gob.NewEncoder(serverConn)
	clientGOBIn, clientGOBOut := gob.NewDecoder(clientConn), gob.NewEncoder(clientConn)

	serverConstFormatter := &constLenFormatter{len: 100, rw: serverConn}
	clientConstFormatter := &constLenFormatter{len: 100, rw: clientConn}

	for i, tt := range []struct {
		encoder interface{ Encode(interface{}) error }
		decoder interface{ Decode(interface{}) error }
	}{
		{encoder: serverJSONOut, decoder: clientJSONIn},
		{encoder: clientJSONOut, decoder: serverJSONIn},
		{encoder: serverGOBOut, decoder: clientGOBIn},
		{encoder: clientGOBOut, decoder: serverGOBIn},
		{encoder: serverConstFormatter, decoder: clientConstFormatter},
		{encoder: clientConstFormatter, decoder: serverConstFormatter},
	} {
		require.NoError(t, tt.encoder.Encode(i))
		var answer int
		require.NoError(t, tt.decoder.Decode(&answer))
		assert.Equal(t, i, answer)
	}
}

func startServer() (server *httptest.Server, serverAccepted <-chan *Conn, serverHandlerWait chan<- struct{}) {
	var (
		accepted    = make(chan *Conn)
		handlerWait = make(chan struct{})
	)

	server = h2test.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		serverConn, err := Accept(w, r)
		if err != nil {
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
		accepted <- serverConn
		<-handlerWait
	}))

	return server, accepted, handlerWait
}

func nopHandler(t *testing.T) *httptest.Server {
	return h2test.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := Accept(w, r)
		require.NoError(t, err)
	}))
}

// constLenFormatter is a simple int encoder/decoder that uses reads and writes with constant size.
type constLenFormatter struct {
	len int
	rw  io.ReadWriter
}

func (f *constLenFormatter) Encode(v interface{}) error {
	i, ok := v.(int)
	if !ok {
		return fmt.Errorf("works only for int")
	}
	buf := make([]byte, f.len)

	binary.LittleEndian.PutUint64(buf, uint64(i))

	_, err := f.rw.Write(buf)
	return err
}

func (f *constLenFormatter) Decode(v interface{}) error {
	i, ok := v.(*int)
	if !ok {
		return fmt.Errorf("works only for string pointers")
	}
	buf := make([]byte, f.len)
	n, err := f.rw.Read(buf)
	if err != nil {
		return err
	}

	*i = int(binary.LittleEndian.Uint64(buf[:n]))

	return nil
}
