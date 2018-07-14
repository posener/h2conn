package h2conn

import (
	"context"
	"fmt"
	"io"
	"net"
	"strconv"
	"sync"
	"time"
)

// Conn is client/server symmetric connection.
// It implements the io.Reader/io.Writer/io.Closer to read/write or close the connection to the other side.
// It also has a Send/Recv function to use channels to communicate with the other side.
type Conn struct {
	r  io.Reader
	wc io.WriteCloser

	// addresses
	local, remote net.TCPAddr

	ctx    context.Context
	cancel context.CancelFunc

	wLock sync.Mutex
	rLock sync.Mutex
}

func (c *Conn) Done() <-chan struct{} {
	return c.ctx.Done()
}

func newConn(ctx context.Context, r io.Reader, wc io.WriteCloser, local, remote string) *Conn {
	ctx, cancel := context.WithCancel(ctx)

	c := &Conn{
		r:  r,
		wc: wc,

		ctx:    ctx,
		cancel: cancel,

		local:  netAddr(local),
		remote: netAddr(remote),
	}

	if deadline, ok := ctx.Deadline(); ok {
		c.SetDeadline(deadline)
	}

	return c
}

func (c *Conn) Write(data []byte) (int, error) {
	c.wLock.Lock()
	defer c.wLock.Unlock()
	return c.wc.Write(data)
}

func (c *Conn) Read(data []byte) (int, error) {
	c.rLock.Lock()
	defer c.rLock.Unlock()
	return c.r.Read(data)
}

func (c *Conn) Close() error {
	c.cancel()
	return c.wc.Close()
}

func (c *Conn) LocalAddr() net.Addr {
	return &c.local
}

func (c *Conn) RemoteAddr() net.Addr {
	return &c.remote
}

func (c *Conn) SetDeadline(t time.Time) error {
	return fmt.Errorf("deadline not supported")
}

func (c *Conn) SetReadDeadline(t time.Time) error {
	return fmt.Errorf("deadline not supported")
}

func (c *Conn) SetWriteDeadline(t time.Time) error {
	return fmt.Errorf("deadline not supported")
}

func netAddr(addr string) net.TCPAddr {
	host, portStr, err := net.SplitHostPort(addr)
	if err != nil {
		host = "0.0.0.0"
	}
	port, err := strconv.Atoi(portStr)
	if err != nil {
		port = 80
	}
	return net.TCPAddr{
		IP:   net.ParseIP(host),
		Port: port,
	}
}
