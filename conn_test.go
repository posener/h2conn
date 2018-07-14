package h2conn

import (
	"bytes"
	"context"
	"crypto/tls"
	"log"
	"net/http"
	"sync"
	"testing"
	"time"

	"github.com/posener/h2conn/http2test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const (
	numRequests   = 100
	shortDuration = 100 * time.Millisecond
)

func TestConcurrent(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		serve      func(*Conn)
		clientSend func(*testing.T, *Conn, []byte)
		clientRec  func(*testing.T, *Conn) []byte
	}{
		{
			name:       "channels",
			serve:      serveWithChannel,
			clientSend: clientSendWithChannel,
			clientRec:  clientRecWithChannel,
		},
		{
			name:       "read write",
			serve:      serveWithReadWrite,
			clientSend: clientSendWithWrite,
			clientRec:  clientRecWithRead,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			// serverDone indicates if the server finished serving the client after the client closed the connection
			serverDone := make(chan struct{})

			server := http2test.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				conn, err := New(w, r)
				if err != nil {
					http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
					return
				}
				tt.serve(conn)
				close(serverDone)
			}))
			defer server.Close()

			client, resp, err := Dial(context.Background(), server.URL, &tls.Config{InsecureSkipVerify: true})
			require.Nil(t, err)

			assert.Equal(t, http.StatusCreated, resp.StatusCode)

			var wg sync.WaitGroup
			wg.Add(numRequests)

			go func() {
				for i := 0; i < numRequests; i++ {
					tt.clientSend(t, client, []byte("hello"))
				}
			}()

			go func() {
				for i := 0; i < numRequests; i++ {
					msg := tt.clientRec(t, client)
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
		})
	}
}

func serveWithChannel(conn *Conn) {
	for msg := range conn.Receive {
		conn.Send <- bytes.ToUpper(msg)
	}
}

func clientSendWithChannel(t *testing.T, conn *Conn, msg []byte) {
	conn.Send <- msg
}

func clientRecWithChannel(t *testing.T, conn *Conn) []byte {
	select {
	case msg := <-conn.Receive:
		return msg
	case <-time.After(shortDuration):
		t.Fatalf("Did not receive a response after %s", shortDuration)
	}
	return nil
}

func serveWithReadWrite(conn *Conn) {
	for {
		msg := make([]byte, 100)
		n, err := conn.Read(msg)
		if err != nil {
			log.Printf("Server failed read: %s", err)
			break
		}
		msg = msg[:n]

		_, err = conn.Write(bytes.ToUpper(msg))
		if err != nil {
			log.Printf("Server failed write: %s", err)
			break
		}
	}
}

func clientSendWithWrite(t *testing.T, conn *Conn, msg []byte) {
	_, err := conn.Write(msg)
	require.Nil(t, err)
}

func clientRecWithRead(t *testing.T, conn *Conn) []byte {
	msg := make([]byte, 100)
	n, err := conn.Read(msg)
	require.Nil(t, err)
	return msg[:n]
}
