package h2conn

import (
	"context"
	"encoding/gob"
	"encoding/json"
	"io"
	"sync"
)

// Conn is client/server symmetric connection.
// It implements the io.Reader/io.Writer/io.Closer to read/write or close the connection to the other side.
// It also has a Send/Recv function to use channels to communicate with the other side.
type Conn struct {
	r  io.Reader
	wc io.WriteCloser

	ctx    context.Context
	cancel context.CancelFunc

	wLock sync.Mutex
	rLock sync.Mutex
}

// Done returns a channel that is closed when the other side closes the connection.
func (c *Conn) Done() <-chan struct{} {
	return c.ctx.Done()
}

func newConn(ctx context.Context, r io.Reader, wc io.WriteCloser) *Conn {
	ctx, cancel := context.WithCancel(ctx)

	c := &Conn{
		r:  r,
		wc: wc,

		ctx:    ctx,
		cancel: cancel,
	}

	return c
}

// Write writes data to the connection
func (c *Conn) Write(data []byte) (int, error) {
	c.wLock.Lock()
	defer c.wLock.Unlock()
	return c.wc.Write(data)
}

// Read reads data from the connection
func (c *Conn) Read(data []byte) (int, error) {
	c.rLock.Lock()
	defer c.rLock.Unlock()
	return c.r.Read(data)
}

// Close closes the connection
func (c *Conn) Close() error {
	c.cancel()
	return c.wc.Close()
}

// JSON returns a json encoder and decoder to talk with JSON messages.
// The usage is the same in the client side and in the server side.
// Any type can be sent over the json format, here an example of communicating with string.
// It is important that the client and the server will communicate with the same format
//
// Usage:
//
//		// in receives data from the other side, out sends data to the other side.
//		in, out = conn.JSON()
//
//		err = out.Encode("hello")
//		// handler err
//
//		var resp string
//		err = in.Decode(&resp)
//		// handle err
//
func (c *Conn) JSON() (*json.Decoder, *json.Encoder) {
	return json.NewDecoder(c), json.NewEncoder(c)
}

// GOB returns a GOB format encoder and decoder to talk with GOB messages.
// The usage is the same in the client side and in the server side.
// Any type can be sent over the GOB format, here an example of communicating with string.
// It is important that the client and the server will communicate with the same format
//
// Usage:
//
//		// in receives data from the other side, out sends data to the other side.
//		in, out = conn.GOB()
//
//		err = out.Encode("hello")
//		// handler err
//
//		var resp string
//		err = in.Decode(&resp)
//		// handle err
//
func (c *Conn) GOB() (*gob.Decoder, *gob.Encoder) {
	return gob.NewDecoder(c), gob.NewEncoder(c)
}
