package h2conn

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"log"
	"net"
	"net/http"
	"os"
	"sync"
	"testing"
	"time"

	"github.com/posener/h2conn/h2test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/http2"
	"golang.org/x/net/nettest"
)

const (
	numRequests   = 100
	shortDuration = 100 * time.Millisecond
)

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

	d := &Client{
		Client: &http.Client{
			Transport: &http2.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}},
		},
	}

	clientConn, resp, err := d.Connect(context.Background(), server.URL)
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

	d := &Client{
		Client: &http.Client{
			Transport: &http2.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}},
		},
	}

	clientConn, resp, err := d.Connect(ctx, server.URL)
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
