package h2conn

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"log"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"sync"
	"testing"
	"time"

	"io"

	"github.com/posener/h2conn/h2test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/http2"
	"golang.org/x/net/nettest"
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

	// test that server is closing the connection
	clientConn.Close()
	select {
	case <-serverConn.Done():
	case <-time.After(shortDuration):
		t.Fatalf("Server not done after %s", shortDuration)
	}
}

// TestClientClose tests that server gets io.EOF after client closed the connection
func TestClientClose(t *testing.T) {
	t.Parallel()

	var (
		serverConn        *Conn
		serverHandlerWait = make(chan struct{})
		serverAccepted    = make(chan struct{})
	)

	server := h2test.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var err error
		serverConn, err = Accept(w, r)
		if err != nil {
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
		close(serverAccepted)
		<-serverHandlerWait
	}))
	defer server.Close()

	clientConn, resp, err := insecureClient.Connect(context.Background(), server.URL)
	require.Nil(t, err)

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	// close client connection
	clientConn.Close()

	<-serverAccepted

	// test that read from server returns an io.EOF error
	var buf = make([]byte, 100)
	n, err := serverConn.Read(buf)
	assert.Equal(t, io.EOF, err)
	assert.Equal(t, 0, n)

	// release the server handler wait test that server is closing the connection
	close(serverHandlerWait)
	select {
	case <-serverConn.Done():
	case <-time.After(shortDuration):
		t.Fatalf("Server not done after %s", shortDuration)
	}
}

// TestServer tests that client gets io.EOF after server closed the connection
func TestServerClose(t *testing.T) {
	t.Parallel()

	var (
		serverConn        *Conn
		serverHandlerWait = make(chan struct{})
		serverAccepted    = make(chan struct{})
	)

	server := h2test.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var err error
		serverConn, err = Accept(w, r)
		if err != nil {
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
		close(serverAccepted)
		<-serverHandlerWait
	}))
	defer server.Close()

	clientConn, resp, err := insecureClient.Connect(context.Background(), server.URL)
	require.Nil(t, err)

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	<-serverAccepted

	// close server connection
	serverConn.Close()
	close(serverHandlerWait)

	// test that read from server returns an io.EOF error
	var buf = make([]byte, 100)
	n, err := clientConn.Read(buf)
	assert.Equal(t, io.EOF, err)
	assert.Equal(t, 0, n)

	// release the server handler wait test that server is closing the connection
	select {
	case <-serverConn.Done():
	case <-time.After(shortDuration):
		t.Fatalf("Server not done after %s", shortDuration)
	}
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

func nopHandler(t *testing.T) *httptest.Server {
	return h2test.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, err := Accept(w, r)
		require.NoError(t, err)
	}))
}

func TestFormatters(t *testing.T) {
	t.Parallel()

	var (
		serverConn        *Conn
		serverHandlerWait = make(chan struct{})
		serverAccepted    = make(chan struct{})
	)

	server := h2test.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var err error
		serverConn, err = Accept(w, r)
		if err != nil {
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
		close(serverAccepted)
		<-serverHandlerWait
	}))
	defer server.Close()

	clientConn, resp, err := insecureClient.Connect(context.Background(), server.URL)
	require.Nil(t, err)

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	<-serverAccepted

	var answer int

	serverJSONIn, serverJSONOut := serverConn.JSON()
	clientJSONIn, clientJSONOut := clientConn.JSON()

	serverGOBIn, serverGOBOut := serverConn.GOB()
	clientGOBIn, clientGOBOut := clientConn.GOB()

	require.NoError(t, serverJSONOut.Encode(1))
	require.NoError(t, clientJSONIn.Decode(&answer))
	assert.Equal(t, 1, answer)

	require.NoError(t, clientJSONOut.Encode(2))
	require.NoError(t, serverJSONIn.Decode(&answer))
	assert.Equal(t, 2, answer)

	require.NoError(t, serverGOBOut.Encode(3))
	require.NoError(t, clientGOBIn.Decode(&answer))
	assert.Equal(t, 3, answer)

	require.NoError(t, clientGOBOut.Encode(4))
	require.NoError(t, serverGOBIn.Decode(&answer))
	assert.Equal(t, 4, answer)

	// close server connection
	serverConn.Close()
	close(serverHandlerWait)

	// release the server handler wait test that server is closing the connection
	select {
	case <-serverConn.Done():
	case <-time.After(shortDuration):
		t.Fatalf("Server not done after %s", shortDuration)
	}
}

// TestConn runs the nettest.TestConn on a pipe between an HTTP2 server and client
func TestConn(t *testing.T) {
	// Only TestConn/BasicIO and TestConn/PingPong currently pass
	// as they don't test deadlines.
	// In order to run the tests run:
	// `TEST_CONN=1 go test -race -v -run "TestConn/(BasicIO|PingPong)"`
	if os.Getenv("TEST_CONN") == "" {
		t.Skip("Only TestConn/BasicIO and TestConn/PingPong are passing since there is no deadline support")
	}
	nettest.TestConn(t, func() (c1 net.Conn, c2 net.Conn, stop func(), err error) {
		c1, c2, stop, err = makePipe(t)
		return
	})
}

func makePipe(t *testing.T) (net.Conn, net.Conn, func(), error) {
	ctx, cancel := context.WithCancel(context.Background())
	var serverConn *Conn

	server := h2test.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var err error
		serverConn, err = Accept(w, r)
		require.Nil(t, err)
		<-serverConn.Done()
	}))

	clientConn, resp, err := insecureClient.Connect(ctx, server.URL)
	require.Nil(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	stop := func() {
		server.Close()
		cancel()
	}

	return connWrapper{Conn: serverConn}, connWrapper{Conn: clientConn}, stop, nil
}

type connWrapper struct {
	*Conn
}

func (c connWrapper) LocalAddr() net.Addr {
	return mockAddr{}
}

func (c connWrapper) RemoteAddr() net.Addr {
	return mockAddr{}
}

func (c connWrapper) SetDeadline(t time.Time) error {
	panic("not implemented")
}

func (c connWrapper) SetWriteDeadline(t time.Time) error {
	panic("not implemented")
}

func (c connWrapper) SetReadDeadline(t time.Time) error {
	panic("not implemented")
}

type mockAddr struct{}

func (mockAddr) Network() string { return "mock" }
func (mockAddr) String() string  { return "mock" }
