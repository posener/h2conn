package h2conn

import (
	"bufio"
	"bytes"
	"context"
	"crypto/tls"
	"log"
	"net"
	"net/http"
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

func TestConcurrent(t *testing.T) {
	t.Parallel()

	// serverDone indicates if the server finished serving the client after the client closed the connection
	serverDone := make(chan struct{})

	server := h2test.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		conn, err := Upgrade(w, r)
		if err != nil {
			http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
			return
		}
		buf := bufio.NewReader(conn)
		for {
			msg, _, err := buf.ReadLine()
			if err != nil {
				log.Printf("Server failed read: %s", err)
				break
			}

			_, err = conn.Write(append(bytes.ToUpper(msg), '\n'))
			if err != nil {
				log.Printf("Server failed write: %s", err)
				break
			}
		}
		close(serverDone)
	}))
	defer server.Close()

	d := &Dialer{
		Client: &http.Client{
			Transport: &http2.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}},
		},
	}

	client, resp, err := d.Dial(context.Background(), server.URL, nil)
	require.Nil(t, err)

	assert.Equal(t, http.StatusOK, resp.StatusCode)

	buf := bufio.NewReader(client)

	var wg sync.WaitGroup
	wg.Add(numRequests)

	go func() {
		for i := 0; i < numRequests; i++ {
			_, err := client.Write([]byte("hello\n"))
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
	client.Close()
	select {
	case <-serverDone:
	case <-time.After(shortDuration):
		t.Fatalf("Server not done after %s", shortDuration)
	}
}

func TestConn(t *testing.T) {
	t.Skip("Only TestConn/BasicIO and TestConn/PingPong are passing since there is no deadline support")
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
		serverConn, err = Upgrade(w, r)
		require.Nil(t, err)
		<-serverConn.Done()
	}))

	d := &Dialer{
		Client: &http.Client{
			Transport: &http2.Transport{TLSClientConfig: &tls.Config{InsecureSkipVerify: true}},
		},
	}

	clientConn, resp, err := d.Dial(ctx, server.URL, nil)
	require.Nil(t, err)
	assert.Equal(t, http.StatusOK, resp.StatusCode)

	stop := func() {
		server.Close()
		cancel()
	}

	return serverConn, clientConn, stop, nil
}
