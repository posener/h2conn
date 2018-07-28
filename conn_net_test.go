package h2conn

import (
	"context"
	"net"
	"net/http"
	"os"
	"testing"
	"time"

	"github.com/posener/h2conn/h2test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"golang.org/x/net/nettest"
)

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
	nettest.TestConn(t, func() (c1 net.Conn, c2 net.Conn, stop func(), err error) {
		c2, c1, stop, err = makePipe(t)
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
