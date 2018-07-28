# h2conn

`h2conn` is an HTTP2 client-server connection, similar to websockets but over HTTP2.

[![Build Status](https://travis-ci.org/posener/h2conn.svg?branch=master)](https://travis-ci.org/posener/h2conn)
[![codecov](https://codecov.io/gh/posener/h2conn/branch/master/graph/badge.svg)](https://codecov.io/gh/posener/h2conn)
[![GoDoc](https://godoc.org/github.com/posener/h2conn?status.svg)](http://godoc.org/github.com/posener/h2conn)
[![Go Report Card](https://goreportcard.com/badge/github.com/posener/h2conn)](https://goreportcard.com/report/github.com/posener/h2conn)

Get a connection object that provides [`io.ReadWriteCloser`](https://golang.org/pkg/io/#ReadWriteCloser)
interface over an HTTP2 connection, For easy, full-duplex communication.

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

On the server side, in an implementation of `http.Handler`, the `h2conn.Accept` function
can be used to get a full-duplex connection to the client.


```go
type handler struct{}

func (h handler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	conn, err := h2conn.Accept(w, r)
	if err != nil {
		log.Printf("Failed creating connection from %s: %s", r.RemoteAddr, err)
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}
	defer conn.Close() 
	
	// [ Use conn ... ]
	// The connection will be left open until this function will return.
	// If there is a need to wait to the client to close the connection,
	// we can wait on the request context: `<-r.Context().Done()`.
}
```

### Client

On the client side, the `h2conn.Connect` function can be used in order to connect to an HTTP2 server
with full-duplex communication.

```go
func main() {
    conn, resp, err := h2conn.Connect(ctx, url, nil)
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

### Using the connection

The server and the client need to decide on message format.
Here are few examples that demonstrate how the client and server can communicate over the created pipe.

1. Simple constant read and write buffer size can be used.

```go
// Create constant size buffer
const bufSize = 10

func main () {
	// [ Create a connection ... ]
	
    buf := make([]byte, bufSize)

    // Write to the connection:
    // [ Write something to buf... ]
    _, err = conn.Write(buf)

    // Read from the connection:
    _, err = conn.Read(buf)
    // [ Use buf... ]
}
```

2. Sending and receiving JSON format is a very common thing to do:

```go
import "encoding/json"

func main() {
	// [ Create a connection ... ]
	
	// Create an encoder and decoder from the connection
	var in, out = json.NewDecoder(conn), json.NewEncoder(conn)
	
    // Sending data into the connection using the out encoder.	
    // Any type can be sent - the important thing is that the other side will read with a
    // variable of the same type.
    // In this case, we just use a simple string.
	err = out.Encode("hello")
	// [ handle err ... ]
	
	// Receiving data from the connection using the in decoder and a variable.
    // Any type can be received - the important thing is that the other side will write data
    // to the connection of the same type.
	var resp string
    err = in.Decode(&resp)	
    // [ handle err, use resp ... ]
}
```

3. GOB is more efficient message format, but requires both client and server to be written in Go.
   The example is exactly the same as in the json encoding, just switch `json` with `gob`.

```go
import "encoding/gob"

func main() {
	// [ Create a connection ... ]
	
	var in, out = gob.NewDecoder(conn), gob.NewEncoder(conn)
	
    // Sending data into the connection using the out encoder.	
    // Any type can be sent - the important thing is that the other side will read with a
    // variable of the same type.
    // In this case, we just use a simple string.
	err = out.Encode("hello")
	// [ handle err ... ]
	
	// Receiving data from the connection using the in decoder and a variable.
    // Any type can be received - the important thing is that the other side will write data
    // to the connection of the same type.
	var resp string
    err = in.Decode(&resp)	
    // [ handle err, use resp ... ]
}
```
