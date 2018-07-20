package h2conn

import (
	"context"
	"fmt"
	"io"
	"sync"
	"time"
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

// SetWriteDeadLine sets write deadline for the connection
// it is currently not supported
func (c *Conn) SetWriteDeadline(t time.Time) error {
	return fmt.Errorf("deadline not supported")
}
