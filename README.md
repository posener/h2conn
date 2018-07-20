# h2conn

`h2conn` is an HTTP2 client-server connection, similar to websockets but over HTTP2.

[![Build Status](https://travis-ci.org/posener/h2conn.svg?branch=master)](https://travis-ci.org/posener/h2conn)
[![codecov](https://codecov.io/gh/posener/h2conn/branch/master/graph/badge.svg)](https://codecov.io/gh/posener/h2conn)
[![GoDoc](https://godoc.org/github.com/posener/h2conn?status.svg)](http://godoc.org/github.com/posener/h2conn)
[![Go Report Card](https://goreportcard.com/badge/github.com/posener/h2conn)](https://goreportcard.com/report/github.com/posener/h2conn)

Get an implementation of [`net.Conn`](https://godoc.org/net#Conn) on both the client and server sides from
an HTTP2 connection, For easy, full-duplex communication.

> * The returned connection does not implement the deadline functions.

## Motivation

Go has a wonderful HTTP2 support that is integrated seamlessly into the HTTP1.1 implementation.
There is a nice demo on [https://http2.golang.org](https://http2.golang.org) to see it in action.
The code for the demo is available [here](https://github.com/golang/net/tree/master/http2/h2demo).
I was interested how HTTP2 can work with full-duplex communication, Something similar to web-sockets, 
and saw the [echo handler implementation](https://github.com/golang/net/blob/a680a1efc54dd51c040b3b5ce4939ea3cf2ea0d1/http2/h2demo/h2demo.go#L136-L164),
And a client implementation for this handler in this [Github issue](https://github.com/golang/go/issues/13444#issuecomment-161115822).

I thought how I can make this easier, and came out with this library.

## Examples

Check out the [example](https://github.com/posener/h2conn/tree/master/example) directory.

### Server

On the server side, in an implementation of `http.Handler`, the `ht2conn.Upgrade` function
can be used to get a full-duplex connection to the client.


```go
type handler struct{}

func (h handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	conn, err := h2conn.Upgrade(w, r)
	if err != nil {
		log.Printf("Failed creating connection from %s: %s", r.RemoteAddr, err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	defer conn.Close()

    // Use conn...
}
```

### Client

On the client side, the `h2conn.Dial` function can be used in order to connect to an HTTP2 server
with full-duplex communication.

```go
func main() {
    conn, resp, err := h2conn.Dial(ctx, url, nil)
	if err != nil {
		log.Fatalf("Initiate conn: %s", err)
	}
	defer conn.Close()

	// Check server status code
	if resp.StatusCode != http.StatusOK {
		log.Fatalf("Bad status code: %d", resp.StatusCode)
	}

	// Use conn...
}
```
